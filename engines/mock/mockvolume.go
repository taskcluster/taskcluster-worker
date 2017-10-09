package mockengine

import (
	"io"
	"os"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// A mock volume basically hold a bit value that can be set or cleared
type volume struct {
	engines.VolumeBase
	engines.VolumeBuilderBase
	files map[string]string
}

func (v *volume) BuildVolume() (engines.Volume, error) {
	return v, nil
}

func (v *volume) WriteEntry(info os.FileInfo) (io.WriteCloser, error) {
	return nil, nil
}
