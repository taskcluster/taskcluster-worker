package qemubuild

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/taskcluster/taskcluster-worker/commands/qemu-run"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func buildImage(
	monitor runtime.Monitor,
	inputFile, outputFile string,
	fromImage, novnc bool,
	boot,
	cdrom string,
	size int,
) error {
	// Find absolute outputFile
	outputFile, err := filepath.Abs(outputFile)
	if err != nil {
		monitor.Error("Failed to resolve output file, error: ", err)
		return err
	}

	// Create temp folder for the image
	tempFolder, err := ioutil.TempDir("", "taskcluster-worker-build-image-")
	if err != nil {
		monitor.Error("Failed to create temporary folder, error: ", err)
		return err
	}
	defer os.RemoveAll(tempFolder)

	var img *image.MutableImage
	if !fromImage {
		// Read machine definition
		machine, err2 := vm.LoadMachine(inputFile)
		if err2 != nil {
			monitor.Error("Failed to load machine file from ", inputFile, " error: ", err2)
			return err2
		}

		// Construct MutableImage
		monitor.Info("Creating MutableImage")
		img, err2 = image.NewMutableImage(tempFolder, int(size), machine)
		if err2 != nil {
			monitor.Error("Failed to create image, error: ", err2)
			return err2
		}
	} else {
		img, err = image.NewMutableImageFromFile(inputFile, tempFolder)
		if err != nil {
			monitor.Error("Failed to load image, error: ", err)
			return err
		}
	}

	// Create temp folder for sockets
	socketFolder, err := ioutil.TempDir("", "taskcluster-worker-sockets-")
	if err != nil {
		monitor.Error("Failed to create temporary folder, error: ", err)
		return err
	}
	defer os.RemoveAll(socketFolder)

	// Setup a user-space network
	monitor.Info("Creating user-space network")
	net, err := network.NewUserNetwork(tempFolder)
	if err != nil {
		monitor.Error("Failed to create user-space network, error: ", err)
		return err
	}

	// Setup logService so that logs can be posted to meta-service at:
	// http://169.254.169.254/v1/log
	net.SetHandler(&logService{Destination: os.Stdout})

	// Create virtual machine
	monitor.Info("Creating virtual machine")
	vm, err := vm.NewVirtualMachine(img.Machine().Options(), img, net, socketFolder, boot, cdrom, monitor.WithTag("component", "vm"))
	if err != nil {
		monitor.Error("Failed to recreated virtual-machine, error: ", err)
		return err
	}

	// Start the virtual machine
	monitor.Info("Starting virtual machine")
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
		err = errors.New("SIGINT received, aborting virtual machine")
	case <-vm.Done:
		err = vm.Error
	}
	<-vm.Done
	signal.Stop(interrupted)
	defer img.Dispose()

	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			monitor.Error("QEMU error: ", string(e.Stderr))
		}
		monitor.Info("Error running virtual machine: ", err)
		return err
	}

	// Package up the finished image
	monitor.Info("Package virtual machine image")
	err = img.Package(outputFile)
	if err != nil {
		monitor.Error("Failed to package finished image, error: ", err)
		return err
	}

	return nil
}
