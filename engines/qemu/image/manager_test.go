// +build qemu

package image

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

const testImageFile = "../test-image/tinycore-worker.tar.zst"

func TestImageManager(t *testing.T) {
	debug(" - Setup environment needed to test")
	gc := &gc.GarbageCollector{}
	log := logrus.StandardLogger()
	monitor := mocks.NewMockMonitor(true)
	imageFolder := filepath.Join("/tmp", slugid.Nice())

	debug(" - Create manager")
	manager, err := NewManager(imageFolder, gc, log.WithField("subsystem", "image-manager"), monitor)
	require.NoError(t, err, "Failed to create image manager")

	debug(" - Test parallel download")
	// Check that download can return and error, and we won't download twice
	// if we call before returning...
	downloadError := errors.New("test error")
	var err1 error
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		_, err1 = manager.Instance("url:test-image-1", func(target string) error {
			time.Sleep(100 * time.Millisecond) // Sleep giving the second call time
			return downloadError
		})
		wg.Done()
	}()
	time.Sleep(50 * time.Millisecond) // Sleep giving the second call time
	instance, err2 := manager.Instance("url:test-image-1", func(target string) error {
		panic("We shouldn't get here, as the previous download haven't returned")
	})
	wg.Done()
	wg.Wait()
	require.True(t, err1 == err2, "Expected the same errors: ", err1, err2)
	require.True(t, downloadError == err1, "Expected the downloadError: ", err1)
	require.True(t, instance == nil, "Expected instance to nil, when we have an error")

	debug(" - Test instantiation of image")
	instance, err = manager.Instance("url:test-image-1", func(target string) error {
		return copyFile(testImageFile, target)
	})
	require.NoError(t, err, "Failed to loadImage")
	require.True(t, instance != nil, "Expected an instance")

	debug(" - Get the diskImage path so we can check it gets deleted")
	diskImage := instance.DiskFile()

	debug(" - Inspect file for sanity check: ", diskImage)
	info := inspectImageFile(diskImage, imageQCOW2Format)
	require.True(t, info != nil, "Expected a qcow2 file")
	require.True(t, info.Format == formatQCOW2)
	require.True(t, !info.DirtyFlag)
	require.True(t, info.BackingFile != "", "Missing backing file in qcow2")

	debug(" - Check that backing file exists")
	backingFile := filepath.Join(filepath.Dir(diskImage), info.BackingFile)
	_, err = os.Lstat(backingFile)
	require.NoError(t, err, "backingFile missing")

	debug(" - Garbage collect and test that image is still there")
	require.NoError(t, gc.CollectAll(), "gc.CollectAll() failed")
	_, err = os.Lstat(backingFile)
	require.NoError(t, err, "backingFile missing after GC")
	info = inspectImageFile(diskImage, imageQCOW2Format)
	require.True(t, info != nil, "diskImage for instance deleted after GC")

	debug(" - Make a new instance")
	instance2, err := manager.Instance("url:test-image-1", func(target string) error {
		panic("We shouldn't get here, as it is currently in the cache")
	})
	require.NoError(t, err, "Failed to create new instance")
	diskImage2 := instance2.DiskFile()
	require.True(t, diskImage2 != diskImage, "Expected a new disk image")
	info = inspectImageFile(diskImage2, imageQCOW2Format)
	require.True(t, info != nil, "diskImage2 missing initially")

	debug(" - Release the first instance")
	instance.Release()
	_, err = os.Lstat(diskImage)
	require.True(t, os.IsNotExist(err), "first instance diskImage shouldn't exist!")
	info = inspectImageFile(diskImage2, imageQCOW2Format)
	require.True(t, info != nil, "diskImage2 missing after first instance release")

	debug(" - Garbage collect and test that image is still there")
	require.NoError(t, gc.CollectAll(), "gc.CollectAll() failed")
	_, err = os.Lstat(backingFile)
	require.NoError(t, err, "backingFile missing after second GC")
	_, err = os.Lstat(diskImage)
	require.True(t, os.IsNotExist(err), "first instance diskImage shouldn't exist!")
	info = inspectImageFile(diskImage2, imageQCOW2Format)
	require.True(t, info != nil, "diskImage2 missing after first instance release")

	debug(" - Release the second instance")
	instance2.Release()
	_, err = os.Lstat(diskImage2)
	require.True(t, os.IsNotExist(err), "second instance diskImage shouldn't exist!")
	_, err = os.Lstat(backingFile)
	require.NoError(t, err, "backingFile missing after release, this shouldn't be...")

	debug(" - Garbage collect everything") // this should dispose the image
	require.NoError(t, gc.CollectAll(), "gc.CollectAll() failed")
	_, err = os.Lstat(backingFile)
	require.True(t, os.IsNotExist(err), "Expected backingFile to be deleted after GC, file: ", backingFile)

	debug(" - Check that we can indeed reload the image")
	_, err = manager.Instance("url:test-image-1", func(target string) error {
		return downloadError
	})
	require.True(t, err == downloadError, "Expected a downloadError", err)
}
