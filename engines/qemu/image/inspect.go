package image

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
)

type imageFormat int

const (
	imageRawFormat imageFormat = iota
	imageQCOW2Format
)

// Snapshot represents the meta-data for a snapshot in a qcow2 file.
type snapshot struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	StateSize uint64 `json:"vm-state-size"`
}

// Information represents the meta-data for a qcow2 file.
type information struct {
	File          string     `json:"filename"`
	Format        string     `json:"format"`
	VirtualSize   int64      `json:"virtual-size"`
	ClusterSize   int64      `json:"cluster-size"`
	ActualSize    int64      `json:"actual-size"`
	DirtyFlag     bool       `json:"dirty-flag"`
	BackingFormat string     `json:"backing-filename-format"`
	BackingFile   string     `json:"backing-filename"`
	Snapshots     []snapshot `json:"snapshots"`
}

// inspectImageFile reads image meta-data for an image file of type
func inspectImageFile(imageFile string, format imageFormat) *information {
	fmt := "raw"
	if format == imageQCOW2Format {
		fmt = "qcow2"
	}
	p := exec.Command("qemu-img", "-f", fmt, "--output", "json", filepath.Base(imageFile))
	p.Dir = filepath.Dir(imageFile)
	data, err := p.Output()
	if err != nil {
		return nil
	}
	info := &information{}
	err = json.Unmarshal(data, info)
	if err != nil {
		return nil
	}
	return info
}
