package image

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

// Manager loads and tracks images.
type Manager struct {
	m           sync.Mutex
	images      map[string]*image
	imageFolder string
	gc          *gc.GarbageCollector
}

// image represents an image of which multiple instances can be created
type image struct {
	gc.DisposableResource
	imageID string
	folder  string
	machine *Machine
	done    <-chan struct{}
	manager *Manager
	err     error
}

// Instance represents an instance of an image.
type Instance struct {
	m        sync.Mutex
	image    *image
	diskFile string
}

// NewManager creates a new image manager using the imageFolder for storing
// images and instances of images.
func NewManager(imageFolder string, gc *gc.GarbageCollector) *Manager {
	// Ensure the image folder is created
	err := os.MkdirAll(imageFolder, 0600)
	if err != nil {
		panic(fmt.Sprint("Failed to create imageFolder: ", imageFolder, ", error: ", err))
	}
	return &Manager{
		images:      make(map[string]*image),
		imageFolder: imageFolder,
		gc:          gc,
	}
}

// NewInstance will return an Instance of the image with imageID. If no such
// image exists in the cache, download() will be called to download it to a
// temporary filename.
//
// This method will insert the downloaded image into the cache, and ensures that
// if won't be downloaded twice, if another invocation already is downloading
// an image with the same imageID.
//
// It is the responsibility of the caller to make sure that imageID is a string
// that uniquely identifies the image. Sane patterns includes "url:<url>", or
// "taskId:<taskId>/<runId>/<artifact>". It also the callers responsibility to
// enforce any sort of access control.
func (m *Manager) NewInstance(imageID string, download func(imageFile string) error) (*Instance, error) {
	m.m.Lock()

	// Get image from cache and insert it if not present
	img := m.images[imageID]
	if img == nil {
		imageDone := make(chan struct{})
		img = &image{
			imageID: imageID,
			folder:  filepath.Join(m.imageFolder, slugid.V4()),
			done:    imageDone,
			manager: m,
		}
		m.images[imageID] = img
		// Start loading the image
		go img.loadImage(download, imageDone)
	}

	// Acqure the image, so we can release lock without risking the image gets
	// garbage collected.
	img.Acquire()
	m.m.Unlock() // Release lock we don't need it anymore

	// Wait for image to be done, then either return the error, or return an
	// instance of the image.
	<-img.done
	if img.err != nil {
		img.Release()
		return nil, img.err
	}
	return img.instance(), nil
}

func (img *image) loadImage(download func(imageFile string) error, done chan<- struct{}) {
	imageFile := filepath.Join(img.manager.imageFolder, slugid.V4()+".tar")

	// Create image folder
	err := os.Mkdir(img.folder, 0600)
	if err != nil {
		goto cleanup
	}

	// Download image to tempory file
	err = download(imageFile)
	if err != nil {
		goto cleanup
	}

	// Extract image and validate image
	img.machine, err = extractImage(imageFile, img.folder)
	if err != nil {
		goto cleanup
	}

	// Clean up if there is any error
cleanup:
	// Delete the image file
	err2 := os.RemoveAll(imageFile)
	if err2 != nil {
		// TODO: Log a message to the log with err2
	}

	// If there was an err, set ima.err and remove it from cache
	if err != nil {
		img.err = err
		// We should always remove a failed attempt from the cache
		img.manager.m.Lock()
		delete(img.manager.images, img.imageID)
		img.manager.m.Unlock()

		// Delete the image folder
		err2 := os.RemoveAll(img.folder)
		if err2 != nil {
			// TODO: Log a message to the log with err2
		}
	} else {
		img.manager.gc.Register(img)
	}
	close(done)
}

func (img *image) Dispose() error {
	// Lock image manager, so we can remove the entry, and ensure that we don't
	// have a race condition between CanDispose and someone calling Acquire()
	img.manager.m.Lock()
	defer img.manager.m.Unlock()

	// Don't dispose if we can't dispose
	if err := img.CanDispose(); err != nil {
		return err
	}

	// Check that we're not disposing twice
	if _, ok := img.manager.images[img.imageID]; !ok {
		panic("Can't dispose an image twice")
	}
	// Remove image entry
	delete(img.manager.images, img.imageID)

	// Delete the image folder
	if err := os.RemoveAll(img.folder); err != nil {
		return fmt.Errorf("Failed to delete image folder '%s', error: %s", img.folder, err)
	}

	return nil
}

// instance returns a new instance of the image for use in a virtual machine.
// You must have called image.Acquire() first to prevent garbage collection.
func (img *image) instance() *Instance {
	// Create a copy of layer.qcow2
	diskFile := filepath.Join(img.manager.imageFolder, slugid.V4()+".qcow2")
	err := copyFile(filepath.Join(img.manager.imageFolder, "layer.qcow2"), diskFile)
	if err != nil {
		panic(fmt.Sprint("Failed to make copy of layer.qcow2, error: ", err))
	}

	return &Instance{
		image:    img,
		diskFile: diskFile,
	}
}

// Machine returns the virtual machine configuration for this instance.
func (i *Instance) Machine() Machine {
	i.m.Lock()
	defer i.m.Unlock()
	return *i.image.machine
}

// DiskFile returns the qcow2 file this image instance is backed by.
func (i *Instance) DiskFile() string {
	i.m.Lock()
	defer i.m.Unlock()
	if i.image == nil {
		panic("Instance of image is already disposed")
	}
	return ""
}

// Release frees the resources held by an instance.
func (i *Instance) Release() {
	i.m.Lock()
	defer i.m.Unlock()
	if i.image == nil {
		panic("Instance of image is already disposed")
	}

	// Delete the layer.qcow2 copy
	if err := os.Remove(i.diskFile); err != nil {
		// TODO: Log err somewhere... ideally to sentry!
	}

	// Release the image
	i.image.Release()
	i.image = nil // ensure that we never do this twice
}
