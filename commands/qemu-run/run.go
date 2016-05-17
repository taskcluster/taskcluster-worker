package qemurun

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/cespare/cp"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

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

func (cmd) Execute(arguments map[string]interface{}) {
	// Read arguments
	imageFile := arguments["<image>"].(string)
	command := arguments["<command>"].([]string)
	vnc := arguments["--vnc"].(bool)

	// Create a temporary folder
	tempFolder := filepath.Join("/tmp", slugid.V4())
	if err := os.Mkdir(tempFolder, 0777); err != nil {
		log.Fatal("Failed to create temporary folder in /tmp, error: ", err)
	}

	// Create the necessary runtime setup
	gc := &gc.GarbageCollector{}
	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "qemu-run")

	// Create image manager
	log.Info("Creating image manager")
	manager, err := image.NewManager(filepath.Join(tempFolder, "/images/"), gc, logger.WithField("component", "image-manager"), nil)
	if err != nil {
		log.Fatal("Failed to create image manager", err)
	}

	// Get an instance of the image
	log.Info("Creating instance of image")
	image, err := manager.Instance("image", func(target string) error {
		return cp.CopyFile(target, imageFile)
	})
	if err != nil {
		log.Fatal("Failed to create instance of image, error: ", err)
	}

	// Setup a user-space network
	log.Info("Creating user-space network")
	net, err := network.NewUserNetwork(tempFolder)
	if err != nil {
		log.Fatal("Failed to create user-space network, error: ", err)
	}

	// Create virtual machine
	log.Info("Creating virtual machine")
	vm := vm.NewVirtualMachine(image, net, tempFolder)

	// Create meta-data service
	log.Info("Creating meta-data service")
	ms := metaservice.New(command, make(map[string]string), os.Stdout, func(result bool) {
		fmt.Println("### Task Completed: ", result)
	})

	// Setup http handler
	vm.SetHTTPHandler(ms)

	// Start the virtual machine
	log.Info("Start the virtual machine")
	vm.Start()

	// Start vncviewer
	done := make(chan struct{})
	if vnc {
		go StartVNCViewer(vm.VNCSocket(), done)
	}

	// Wait for SIGINT/SIGKILL or vm.Done
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, os.Kill) // This pattern leaks, acceptable here
	select {
	case <-c:
		signal.Stop(c)
		fmt.Println("### Terminating QEMU")
		vm.Kill()
	case <-vm.Done:
		fmt.Println("### QEMU terminated, error: ", vm.Error)
	}
	close(done)

	// Ensure that QEMU has terminated before we continue
	<-vm.Done

	// Clean up anything left in the garbage collector
	gc.CollectAll()
}
