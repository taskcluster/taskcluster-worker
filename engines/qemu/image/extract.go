package image

import (
	"crypto/rand"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const maxImageSize = 20 * 1024 * 1024 * 1024

// RandomMAC generates a new random MAC with the local bit set.
func RandomMAC() string {
	// Credits: http://stackoverflow.com/a/21027407/68333
	// Get some random data
	m := make([]byte, 6)
	_, err := rand.Read(m)
	if err != nil {
		panic(err)
	}
	m[0] = (m[0] | 2) & 0xfe // Set local bit, ensure unicast address
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", m[0], m[1], m[2], m[3], m[4], m[5])
}

// extractImage will extract the "disk.img", "layer.qcow2" and "machine.json"
// files from a tar archive using GNU tar ensuring that sparse entries will be
// extracted as sparse files.
//
// This also validates that files aren't symlinks and are in correct format,
// with legal backing_file parameters.
//
// Returns a MalformedPayloadError if we believe extraction failed due to a
// badly formatted image.
func extractImage(imageFile, imageFolder string) (*vm.Machine, error) {
	// Restrict file to some maximum size
	if !ioext.IsPlainFile(imageFile) {
		return nil, fmt.Errorf("extractImage: imageFile is not a file")
	}
	if !ioext.IsFileLessThan(imageFile, maxImageSize) {
		return nil, engines.NewMalformedPayloadError("Image file is larger than ", maxImageSize, " bytes")
	}

	// Using lz4 | tar so we get sparse files (sh to get OS pipes)
	tar := exec.Command("sh", "-fec", "lz4 -dq '"+imageFile+"' - | "+
		"tar -xoC '"+imageFolder+"' --no-same-permissions -- "+
		"disk.img layer.qcow2 machine.json",
	)
	_, err := tar.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, engines.NewMalformedPayloadError(
				"Failed to extract image archieve, error: ", string(ee.Stderr),
			)
		}
		// If this wasn't GNU tar exiting non-zero then it must be some internal
		// error. Perhaps tar is missing from the PATH.
		return nil, fmt.Errorf("Failed to extract image archieve, error: %s", err)
	}

	// Check files exist, are plain files and not larger than maxImageSize
	for _, name := range []string{"disk.img", "layer.qcow2", "machine.json"} {
		f := filepath.Join(imageFolder, name)
		if !ioext.IsPlainFile(f) {
			return nil, engines.NewMalformedPayloadError("Image file is missing '", name, "'")
		}
		if !ioext.IsFileLessThan(f, maxImageSize) {
			return nil, engines.NewMalformedPayloadError("Image file contains '",
				name, "' larger than ", maxImageSize, " bytes")
		}
	}

	// Load the machine configuration
	machineFile := filepath.Join(imageFolder, "machine.json")
	machine, err := vm.LoadMachine(machineFile)
	if err != nil {
		return nil, err
	}

	// Inspect the raw disk file
	diskFile := filepath.Join(imageFolder, "disk.img")
	diskInfo := inspectImageFile(diskFile, imageRawFormat)
	if diskInfo == nil || diskInfo.Format != "raw" {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'disk.img' which is not a RAW image file")
	}
	if diskInfo.VirtualSize > maxImageSize {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'disk.img' has virtual size larger than ", maxImageSize, " bytes")
	}
	if diskInfo.DirtyFlag {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'disk.img' which has the dirty-flag set")
	}
	if diskInfo.BackingFile != "" {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'disk.img' which has a backing file, this is not permitted")
	}

	// Inspect the QCOW2 layer file
	layerFile := filepath.Join(imageFolder, "layer.qcow2")
	layerInfo := inspectImageFile(layerFile, imageQCOW2Format)
	if layerInfo == nil || layerInfo.Format != "qcow2" {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'layer.qcow2' which is not a QCOW2 file")
	}
	if layerInfo.VirtualSize > maxImageSize {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'layer.qcow2' has virtual size larger than ", maxImageSize, " bytes")
	}
	if layerInfo.DirtyFlag {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'layer.qcow2' which has the dirty-flag set")
	}
	if layerInfo.BackingFile != "disk.img" {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'layer.qcow2' which has a backing file that isn't: 'disk.img'")
	}
	if layerInfo.BackingFormat != "raw" {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'layer.qcow2' which has a backing file format that isn't 'raw'")
	}

	return machine, nil
}
