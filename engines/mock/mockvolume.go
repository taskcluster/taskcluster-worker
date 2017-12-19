package mockengine

import (
	"bytes"
	"io"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// A mock volume basically hold a bit value that can be set or cleared
type volume struct {
	engines.VolumeBase
	engines.VolumeBuilderBase
	m     sync.Mutex
	files map[string]string
}

type fileWriter struct {
	*bytes.Buffer
	Name   string
	Volume *volume
}

func (w *fileWriter) Close() error {
	w.Volume.m.Lock()
	defer w.Volume.m.Unlock()

	w.Volume.files[w.Name] = w.String()
	return nil
}

func (v *volume) BuildVolume() (engines.Volume, error) {
	return v, nil
}

func (v *volume) WriteFile(name string) io.WriteCloser {
	return &fileWriter{
		Buffer: bytes.NewBuffer(nil),
		Name:   name,
		Volume: v,
	}
}

func (v *volume) WriteFolder(name string) error {
	return nil
}
