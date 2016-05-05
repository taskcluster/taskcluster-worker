//go:generate go-composite-schema --required start machine.yml generated_payloadschema.go

package image

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/taskcluster/taskcluster-worker/engines"
)

const maxImageSize = 20 * 1024 * 1024 * 1024

// isPlainFile returns an true if filePath is a plain file, not a directory,
// symlink, device, etc.
func isPlainFile(filePath string) bool {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&(os.ModeDir|os.ModeSymlink|os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) == 0
}

// isFileLessThan returns true if filePath is a file less than maxSize
func isFileLessThan(filePath string, maxSize int64) bool {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return false
	}
	return fileInfo.Size() < maxSize
}

// Machine defines the options for a virtual machine.
type Machine struct {
	Network struct {
		Device string `json:"device"` // "rtl8139"
		MAC    string `json:"mac"`    // See RandomMAC()
	} `json:"network"`
	// TODO: Add more options in the future
}

// Validate returns a MalformedPayloadError if the Machine definition isn't
// valid and legal.
func (m *Machine) Validate() error {
	hasError := false
	errors := "Invalid machine defintion in 'machine.json'"
	msg := func(a ...interface{}) {
		errors += "\n" + fmt.Sprint(a...)
		hasError = true
	}

	if m.Network.Device != "rtl8139" {
		msg("network.device must be 'rtl8139', but '", m.Network.Device, "' was given")
	}
	// TODO: validate the MAC address

	return nil
}

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
func extractImage(imageFile, imageFolder string) (*Machine, error) {
	// Restrict file to some maximum size
	if !isPlainFile(imageFile) {
		return nil, fmt.Errorf("extractImage: imageFile is not a file")
	}
	if !isFileLessThan(imageFile, maxImageSize) {
		return nil, engines.NewMalformedPayloadError("Image file is larger than ", maxImageSize, " bytes")
	}

	// Using tar so we get sparse files
	tar := exec.Command("tar", "-C", imageFolder, "-xf", imageFile, "disk.img", "layer.qcow2", "machine.json")
	err := tar.Run()
	if err != nil {
		return nil, engines.NewMalformedPayloadError(
			"Failed to extract 'disk' file from image archieve, error: ", err,
		)
	}

	// Check files exist, are plain files and not larger than maxImageSize
	for _, name := range []string{"disk.img", "layer.qcow2", "machine.json"} {
		f := filepath.Join(imageFolder, name)
		if !isPlainFile(f) {
			return nil, engines.NewMalformedPayloadError("Image file is missing '", name, "'")
		}
		if !isFileLessThan(f, maxImageSize) {
			return nil, engines.NewMalformedPayloadError("Image file contains '",
				name, "' larger than ", maxImageSize, " bytes")
		}
	}

	// Load the machine configuration
	machineFile := filepath.Join(imageFolder, "machine.json")
	if !isFileLessThan(machineFile, 1024*1024) {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"'machine.json' larger than 1MiB. That's not okay for a JSON file.")
	}
	machineData, err := ioutil.ReadFile(machineFile)
	if err != nil {
		return nil, err
	}
	machine := &Machine{}
	err = json.Unmarshal(machineData, machine)
	if err != nil {
		return nil, engines.NewMalformedPayloadError("Image file contains ",
			"invalid JSON in 'machine.json', error: ", err)
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
