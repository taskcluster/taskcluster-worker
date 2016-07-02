package qemubuild

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"

	"github.com/taskcluster/taskcluster-worker/commands/qemu-run"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type cmd struct{}

func (cmd) Summary() string {
	return "Build an image for the QEMU engine"
}

func (cmd) Usage() string {
	return `
taskcluster-worker qemu-build takes

run a given command inside an image to test it,
and give you an VNC viewer to get you into the virtual machine.

usage:
	taskcluster-worker qemu-build [options] from-new <machine.json> <result.tar.lz4>
	taskcluster-worker qemu-build [options] from-image <image.tar.lz4> <result.tar.lz4>

options:
     --no-vnc       Do not open a VNC display.
     --size <size>  Size of the image in GiB [default: 10].
     --boot <file>  File to use as cd-rom 1 and boot medium.
     --cdrom <file> File to use as cd-rom 2 (drivers etc).
  -h --help         Show this screen.
`
}

func (cmd) Execute(arguments map[string]interface{}) {
	// Setup logging
	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "qemu-build")

	// Parse arguments
	inputImageFile, _ := arguments["<image.tar.lz4>"].(string)
	machineFile, _ := arguments["<machine.json>"].(string)
	outputFile := arguments["<result.tar.lz4>"].(string)
	fromNew := arguments["from-new"].(bool)
	fromImage := arguments["from-image"].(bool)
	novnc := arguments["--no-vnc"].(bool)
	boot, _ := arguments["--boot"].(string)
	cdrom, _ := arguments["--cdrom"].(string)
	size, err := strconv.ParseInt(arguments["--size"].(string), 10, 32)
	if err != nil {
		log.Fatal("Couldn't parse --size, error: ", err)
	}
	if size > 80 {
		log.Fatal("Images have a sanity limit of 80 GiB!")
	}

	// Find absolute outputFile
	outputFile, err = filepath.Abs(outputFile)
	if err != nil {
		log.Fatal("Failed to resolve output file, error: ", err)
	}

	// Create temp folder for the image
	tempFolder, err := ioutil.TempDir("", "taskcluster-worker-build-image-")
	if err != nil {
		log.Fatal("Failed to create temporary folder, error: ", err)
	}
	defer os.RemoveAll(tempFolder)

	var img *image.MutableImage
	if fromNew {
		// Read machine definition
		machine, err := vm.LoadMachine(machineFile)
		if err != nil {
			log.Fatal("Failed to load machine file from ", machineFile, " error: ", err)
		}

		// Construct MutableImage
		log.Info("Creating MutableImage")
		img, err = image.NewMutableImage(tempFolder, int(size), machine)
		if err != nil {
			log.Fatal("Failed to create image, error: ", err)
		}
	}
	if fromImage {
		img, err = image.NewMutableImageFromFile(inputImageFile, tempFolder)
		if err != nil {
			log.Fatal("Failed to load image, error: ", err)
		}
	}

	// Create temp folder for sockets
	socketFolder, err := ioutil.TempDir("", "taskcluster-worker-sockets-")
	if err != nil {
		log.Fatal("Failed to create temporary folder, error: ", err)
	}
	defer os.RemoveAll(socketFolder)

	// Setup a user-space network
	log.Info("Creating user-space network")
	net, err := network.NewUserNetwork(tempFolder)
	if err != nil {
		log.Fatal("Failed to create user-space network, error: ", err)
	}

	// Create virtual machine
	log.Info("Creating virtual machine")
	vm := vm.NewVirtualMachine(img, net, socketFolder, boot, cdrom)

	// Start the virtual machine
	log.Info("Starting virtual machine")
	vm.Start()

	// Open VNC display
	if !novnc {
		go qemurun.StartVNCViewer(vm.VNCSocket(), vm.Done)
	}

	// Wait for interrupt to gracefully kill everything
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)

	// Wait for virtual machine to be done, or we get interrupted
	select {
	case <-interrupted:
		vm.Kill()
		err = errors.New("SIGINT recieved, aborting virtual machine")
	case <-vm.Done:
		err = vm.Error
	}
	<-vm.Done
	signal.Stop(interrupted)
	defer img.Dispose()

	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			log.Fatal("QEMU error: ", string(e.Stderr))
		}
		log.Info("Error running virtual machine: ", err)
		return
	}

	// Package up the finished image
	log.Info("Package virtual machine image")
	err = img.Package(outputFile)
	if err != nil {
		log.Fatal("Failed to package finished image, error: ", err)
	}
}
