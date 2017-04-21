package qemurun

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	graceful "gopkg.in/tylerb/graceful.v1"

	"github.com/cespare/cp"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/monitoring"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var debug = util.Debug("qemurun")

type cmd struct{}

func (cmd) Summary() string {
	return "Run a QEMU image for debugging"
}

func (cmd) Usage() string {
	return `
taskcluster-worker qemu-run will run a given command inside an image to test it,
and give you an VNC viewer to get you into the virtual machine.

usage: taskcluster-worker qemu-run [options] <image> -- <command>...

options:
  -V --vnc      Open a VNC display
  -h --help     Show this screen.
`
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	// Read arguments
	imageFile := arguments["<image>"].(string)
	command := arguments["<command>"].([]string)
	vnc := arguments["--vnc"].(bool)

	// Create temporary storage and environment
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}
	environment := &runtime.Environment{
		TemporaryStorage: storage,
	}

	monitor := monitoring.NewLoggingMonitor("info", nil).WithTag("component", "qemu-run")

	// Create a temporary folder
	tempFolder := filepath.Join("/tmp", slugid.Nice())
	if err = os.Mkdir(tempFolder, 0777); err != nil {
		monitor.Panic("Failed to create temporary folder in /tmp, error: ", err)
	}

	// Create the necessary runtime setup
	gc := &gc.GarbageCollector{}

	// Create image manager
	monitor.Info("Creating image manager")
	manager, err := image.NewManager(filepath.Join(tempFolder, "/images/"), gc, monitor.WithTag("component", "image-manager"))
	if err != nil {
		monitor.Panic("Failed to create image manager", err)
	}

	// Get an instance of the image
	monitor.Info("Creating instance of image")
	image, err := manager.Instance("image", func(target string) error {
		return cp.CopyFile(target, imageFile)
	})
	if err != nil {
		monitor.Panic("Failed to create instance of image, error: ", err)
	}

	// Setup a user-space network
	monitor.Info("Creating user-space network")
	net, err := network.NewUserNetwork(tempFolder)
	if err != nil {
		monitor.Panic("Failed to create user-space network, error: ", err)
	}

	// Create virtual machine
	monitor.Info("Creating virtual machine")
	vm, err := vm.NewVirtualMachine(
		image.Machine().Options(), image, net, tempFolder,
		"", "", monitor.WithTag("component", "vm"),
	)
	if err != nil {
		monitor.Panic("Failed to create virtual-machine, error: ", err)
	}

	// Create meta-data service
	monitor.Info("Creating meta-data service")
	var shellServer *interactive.ShellServer
	var displayServer *interactive.DisplayServer
	ms := metaservice.New(command, make(map[string]string), os.Stdout, func(result bool) {
		fmt.Println("### Task Completed, result = ", result)
		shellServer.WaitAndClose()
		displayServer.Abort()
		vm.Kill()
	}, environment)

	// Setup http handler for network
	vm.SetHTTPHandler(ms)

	// Create ShellServer
	shellServer = interactive.NewShellServer(
		ms.ExecShell, monitor.WithTag("component", "shell-server"),
	)

	// Create displayServer
	displayServer = interactive.NewDisplayServer(
		&socketDisplayProvider{socket: vm.VNCSocket()},
		monitor.WithTag("component", "display-server"),
	)

	interactiveHandler := http.NewServeMux()
	interactiveHandler.Handle("/shell/", shellServer)
	interactiveHandler.Handle("/display/", displayServer)
	interactiveServer := graceful.Server{
		Timeout: 30 * time.Second,
		Server: &http.Server{
			Addr:    "localhost:8080",
			Handler: interactiveHandler,
		},
		NoSignalHandling: true,
	}
	go interactiveServer.ListenAndServe()

	// Start the virtual machine
	monitor.Info("Start the virtual machine")
	vm.Start()

	// Start vncviewer
	done := make(chan struct{})
	if vnc {
		go StartVNCViewer(vm.VNCSocket(), done)
	}

	// Wait for SIGINT/SIGKILL or vm.Done
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM) // This pattern leaks, acceptable here
	select {
	case <-c:
		signal.Stop(c)
		fmt.Println("### Terminating QEMU")
		vm.Kill()
	case <-vm.Done:
		fmt.Println("### QEMU terminated")
	}
	close(done)

	// Ensure that QEMU has terminated before we continue
	<-vm.Done
	interactiveServer.Stop(100 * time.Millisecond)

	// Clean up anything left in the garbage collector
	gc.CollectAll()
	return true
}
