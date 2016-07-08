package image

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
)

// MutableImage is an vm.MutableImage implementation that keeps the image
// in a single folder. This can be used for testing images and building new
// images.
type MutableImage struct {
	m       sync.Mutex
	inUse   bool
	folder  string
	machine *vm.Machine
}

// NewMutableImage creates a new blank MutableImage of given size in GiB, and
// using the given machine configuration.
//
// This is used when building new images.
func NewMutableImage(folder string, size int, machine *vm.Machine) (*MutableImage, error) {
	// Do a sanity check on the size
	if size > 80 {
		return nil, errors.New("For sanity we don't allow images larger than 80GiB, ask if you need it")
	}

	// Create sparse file
	truncate := exec.Command(
		"truncate", "--size", strconv.Itoa(size)+"G", filepath.Join(folder, "disk.img"),
	)
	_, err := truncate.Output()
	if err != nil {
		msg := err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			msg = string(ee.Stderr)
		}
		return nil, fmt.Errorf("Failed to create sparse file, error: %s", msg)
	}

	return &MutableImage{
		folder:  folder,
		machine: machine.Clone(),
	}, nil
}

// NewMutableImageFromFile creates a mutable image from an existing compressed
// image tar archieve.
func NewMutableImageFromFile(imageFile, imageFolder string) (*MutableImage, error) {
	// Extract image normally
	machine, err := extractImage(imageFile, imageFolder)
	if err != nil {
		return nil, err
	}

	// Remove layer.qcow2
	if err := os.Remove(filepath.Join(imageFolder, "layer.qcow2")); err != nil {
		// Delete image folder, ignoring errors
		os.RemoveAll(imageFolder)

		// Return the original error
		return nil, fmt.Errorf("Failed to delete layer.qcow2 after extract, err: %s", err)
	}

	return &MutableImage{
		folder:  imageFolder,
		machine: machine,
	}, nil
}

// DiskFile returns path to disk file to use in QEMU.
// This also marks the image as being in-use.
func (img *MutableImage) DiskFile() string {
	img.m.Lock()
	defer img.m.Unlock()
	if img.folder == "" {
		panic("MutableImage have been disposed")
	}
	img.inUse = true

	return filepath.Join(img.folder, "disk.img")
}

// Format returns the image format: 'raw'.
func (img *MutableImage) Format() string {
	return "raw"
}

// Machine returns the vm.Machine definition of the virtual machine.
func (img *MutableImage) Machine() vm.Machine {
	img.m.Lock()
	defer img.m.Unlock()
	if img.folder == "" {
		panic("MutableImage have been disposed")
	}

	return *img.machine
}

// Package will write an lz4 compressed tar archieve of the image to targetFile.
// This method cannot be called the image is in-use.
func (img *MutableImage) Package(targetFile string) error {
	img.m.Lock()
	defer img.m.Unlock()
	if img.folder == "" {
		panic("MutableImage have been disposed")
	}
	if img.inUse {
		panic("MutableImage is currently in-use, Release() must be called first")
	}

	// Create layer.qcow2 file
	layer := exec.Command(
		"qemu-img", "create",
		"-f", "qcow2",
		"-o", "backing_file=disk.img,backing_fmt=raw,lazy_refcounts=on",
		"layer.qcow2",
	)
	layer.Dir = img.folder
	_, err := layer.Output()
	if err != nil {
		msg := err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			msg = string(ee.Stderr)
		}
		return fmt.Errorf("Failed to create layer.qcow2 file, error: %s", msg)
	}

	// Create machine.json file
	data, err := json.Marshal(img.machine)
	if err != nil {
		panic(fmt.Sprintf("Failed to json.Marshal machine.json, err: %s", err))
	}
	file, err := os.Create(filepath.Join(img.folder, "machine.json"))
	if err != nil {
		return fmt.Errorf("Failed to create machine.json, err: %s", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("Failed to write machine.json, err: %s", err)
	}
	file.Close()

	// Create tarball of everything
	tar := exec.Command(
		"tar", "-Scf", "image.tar", "disk.img", "layer.qcow2", "machine.json",
	)
	tar.Dir = img.folder
	if _, err := tar.Output(); err != nil {
		msg := err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			msg = string(ee.Stderr)
		}
		return fmt.Errorf("Failed to create image.tar file, error: %s", msg)
	}

	// lz4 compress everything and write to targetFile
	lz4 := exec.Command(
		//TODO: Support high and low compression
		"lz4", "-z5f", "image.tar", targetFile,
	)
	lz4.Dir = img.folder
	if _, err := lz4.Output(); err != nil {
		msg := err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			msg = string(ee.Stderr)
		}
		return fmt.Errorf("Failed to lz4 compress image file, error: %s", msg)
	}

	// Remove layer.qcow2
	if err := os.Remove(filepath.Join(img.folder, "layer.qcow2")); err != nil {
		return fmt.Errorf("Failed to clean up after packaging, err: %s", err)
	}
	// Remove machine.json
	if err := os.Remove(filepath.Join(img.folder, "machine.json")); err != nil {
		return fmt.Errorf("Failed to clean up after packaging, err: %s", err)
	}
	// Remove image.tar
	if err := os.Remove(filepath.Join(img.folder, "image.tar")); err != nil {
		return fmt.Errorf("Failed to clean up after packaging, err: %s", err)
	}

	return nil
}

// Release marks the MutableImage as no longer in use. This allows Package()
// or Dispose() to be called.
func (img *MutableImage) Release() {
	img.m.Lock()
	defer img.m.Unlock()
	if img.folder == "" {
		panic("MutableImage have been disposed")
	}

	// Mark the image no-longer in use
	img.inUse = false
}

// Dispose will delete all resources hold by the MutableImage
func (img *MutableImage) Dispose() {
	img.m.Lock()
	defer img.m.Unlock()
	if img.folder == "" {
		panic("MutableImage have already been released")
	}
	if img.inUse {
		panic("MutableImage is currently in-use, Release() must be called first")
	}

	// Delete image folder, ignoring errors
	os.RemoveAll(img.folder) // TODO: Write errors to log, or something

	// Clear image folder, to prevent reuse.
	img.folder = ""
}
