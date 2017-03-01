package qemubuild

import (
	"strconv"

	"github.com/taskcluster/taskcluster-worker/runtime"
)

type cmd struct{}

func (cmd) Summary() string {
	return "Build an image for the QEMU engine"
}

func (cmd) Usage() string {
	return `
taskcluster-worker qemu-build takes a machine definition as JSON or an existing
image and two ISO files to mounted as CDs and creates a virtual machine that
will be saved to disk when terminated.

usage:
	taskcluster-worker qemu-build [options] from-new <machine.json> <result.tar.zst>
	taskcluster-worker qemu-build [options] from-image <image.tar.zst> <result.tar.zst>

options:
     --no-vnc       	Do not open a VNC display.
     --size <size>  	Size of the image in GiB [default: 10].
     --boot <file>  	File to use as cd-rom 1 and boot medium.
     --cdrom <file>	 	File to use as cd-rom 2 (drivers etc).
  -h --help         	Show this screen.
`
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	// Setup logging
	monitor := runtime.NewLoggingMonitor("info", nil).WithTag("component", "qemu-build")

	// Parse arguments
	outputFile := arguments["<result.tar.zst>"].(string)
	fromNew := arguments["from-new"].(bool)
	fromImage := arguments["from-image"].(bool)
	novnc := arguments["--no-vnc"].(bool)
	boot, _ := arguments["--boot"].(string)
	cdrom, _ := arguments["--cdrom"].(string)
	size, err := strconv.ParseInt(arguments["--size"].(string), 10, 32)
	if err != nil {
		monitor.Panic("Couldn't parse --size, error: ", err)
	}
	if size > 80 {
		monitor.Panic("Images have a sanity limit of 80 GiB!")
	}
	if fromNew == fromImage {
		panic("Impossible arguments")
	}

	var inputFile string
	if !fromImage {
		inputFile = arguments["<machine.json>"].(string)
	} else {
		inputFile = arguments["<image.tar.zst>"].(string)
	}

	return buildImage(
		monitor, inputFile, outputFile,
		fromImage, novnc, boot, cdrom, int(size),
	) == nil
}
