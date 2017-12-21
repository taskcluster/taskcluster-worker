package qemurun

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	graceful "gopkg.in/tylerb/graceful.v1"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
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
     --vnc <port>         Expose VNC on given port.
     --meta <port>        Expose metadata service on port [default: 8080].
     --keep-alive         Keep the VM running until signal is received.
     --log-level <level>  Log level debug, info, warning, error [default: warning].
  -h --help               Show this screen.
`
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	// Read arguments
	imageFile := arguments["<image>"].(string)
	command := arguments["<command>"].([]string)
	keepAlive := arguments["--keep-alive"].(bool)
	logLevel := arguments["--log-level"].(string)
	var vncPort int64
	var err error
	if vnc, ok := arguments["--vnc"].(string); ok {
		vncPort, err = strconv.ParseInt(vnc, 10, 32)
		if err != nil {
			panic(fmt.Sprint("Couldn't parse --vnc, error: ", err))
		}
	}
	metaPort, err := strconv.ParseInt(arguments["--meta"].(string), 10, 32)
	if err != nil {
		panic(fmt.Sprint("Couldn't parse --meta, error: ", err))
	}

	// Create temporary storage and environment
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}
	environment := &runtime.Environment{
		TemporaryStorage: storage,
	}

	monitor := monitoring.NewLoggingMonitor(logLevel, nil, "").WithTag("component", "qemu-run")

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
	image, err := manager.Instance("image", func(target *os.File) error {
		f, ferr := os.Open(imageFile)
		if ferr != nil {
			return ferr
		}
		defer f.Close()
		_, ferr = io.Copy(target, f)
		return ferr
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
		image.Machine().DeriveLimits(), image, net, tempFolder,
		"", "", vm.LinuxBootOptions{},
		monitor.WithTag("component", "vm"),
	)
	if err != nil {
		monitor.Panic("Failed to create virtual-machine, error: ", err)
	}

	// Create meta-data service
	monitor.Info("Creating meta-data service")
	var shellServer *interactive.ShellServer
	var displayServer *interactive.DisplayServer
	retval := atomics.NewBool(false)
	ms := metaservice.New(command, make(map[string]string), os.Stdout, func(result bool) {
		monitor.Info("Task Completed, result = ", result)
		if !keepAlive {
			retval.Set(result)
			shellServer.WaitAndClose()
			displayServer.Abort()
			vm.Kill()
		}
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
			Addr:    fmt.Sprintf(":%d", metaPort),
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
	if vncPort != 0 {
		go ExposeVNC(vm.VNCSocket(), int(vncPort), done)
	}

	// Wait for SIGINT/SIGKILL or vm.Done
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM) // This pattern leaks, acceptable here
	select {
	case <-c:
		signal.Stop(c)
		monitor.Info("Terminating QEMU")
		vm.Kill()
	case <-vm.Done:
		monitor.Info("QEMU terminated")
	}
	close(done)

	// Ensure that QEMU has terminated before we continue
	<-vm.Done
	interactiveServer.Stop(100 * time.Millisecond)

	// Clean up anything left in the garbage collector
	gc.CollectAll()
	return retval.Get()
}
