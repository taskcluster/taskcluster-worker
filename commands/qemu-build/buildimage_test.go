//+build qemu

package qemubuild

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

func TestBuildImage(t *testing.T) {
	// Setup logging
	monitor := mocks.NewMockMonitor(true)

	inputImageFile, err := filepath.Abs("../../engines/qemu/test-image/tinycore-setup.tar.zst")
	if err != nil {
		panic(err)
	}
	outputFile := filepath.Join(os.TempDir(), slugid.Nice())
	defer os.Remove(outputFile)
	novnc := true
	cdrom := ""

	// Create ISO file to play with
	datadir := filepath.Join(os.TempDir(), slugid.Nice())
	defer os.RemoveAll(datadir)
	err = os.Mkdir(datadir, 0700)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(filepath.Join(datadir, "setup.sh"),
		[]byte("#!/bin/sh\necho 'started';\nsudo poweroff;\n"), 0755)
	if err != nil {
		panic(err)
	}
	isofile := filepath.Join(os.TempDir(), slugid.Nice())
	defer os.Remove(isofile)
	err = exec.Command("genisoimage", "-vJrV", "DATA_VOLUME", "-input-charset", "utf-8", "-o", isofile, datadir).Run()
	if err != nil {
		panic(err)
	}

	err = buildImage(
		monitor, inputImageFile, outputFile,
		true, novnc, isofile, cdrom, 1,
	)
	if err != nil {
		panic(err)
	}
}
