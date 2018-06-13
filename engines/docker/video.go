package dockerengine

import (
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	funk "github.com/thoas/go-funk"
)

type device struct {
	path    string
	claimed bool
}

type videoDeviceManager struct {
	devices []device
}

func newVideoDeviceManager() (*videoDeviceManager, error) {
	matches, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to call filepath.Glob function")
	}

	r := regexp.MustCompile("/dev/video[0-9]+")
	matches = funk.FilterString(matches, r.MatchString)

	devices := make([]device, len(matches))
	for i := range devices {
		devices[i].path = matches[i]
	}

	return &videoDeviceManager{
		devices: devices,
	}, nil
}

func (d *videoDeviceManager) claim() *device {
	for i := range d.devices {
		if !d.devices[i].claimed {
			return &d.devices[i]
		}
	}

	return nil
}

func (d *videoDeviceManager) release(dev *device) {
	dev.claimed = false
}
