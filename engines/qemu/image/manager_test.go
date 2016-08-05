// +build qemu

package image

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

const testImageFile = "../test-image/tinycore-worker.tar.lz4"

func TestImageManager(t *testing.T) {
	fmt.Println(" - Setup environment needed to test")
	gc := &gc.GarbageCollector{}
	log := logrus.StandardLogger()
	sentry, _ := raven.New("")
	imageFolder := filepath.Join("/tmp", slugid.Nice())

	fmt.Println(" - Create manager")
	manager, err := NewManager(imageFolder, gc, log.WithField("subsystem", "image-manager"), sentry)
	nilOrPanic(err, "Failed to create image manager")

	fmt.Println(" - Test parallel download")
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
	assert(err1 == err2, "Expected the same errors: ", err1, err2)
	assert(downloadError == err1, "Expected the downloadError: ", err1)
	assert(instance == nil, "Expected instance to nil, when we have an error")

	fmt.Println(" - Test instantiation of image")
	instance, err = manager.Instance("url:test-image-1", func(target string) error {
		return copyFile(testImageFile, target)
	})
	nilOrPanic(err, "Failed to loadImage")
	assert(instance != nil, "Expected an instance")

	fmt.Println(" - Get the diskImage path so we can check it gets deleted")
	diskImage := instance.DiskFile()

	fmt.Println(" - Inspect file for sanity check: ", diskImage)
	info := inspectImageFile(diskImage, imageQCOW2Format)
	assert(info != nil, "Expected a qcow2 file")
	assert(info.Format == "qcow2")
	assert(!info.DirtyFlag)
	assert(info.BackingFile != "", "Missing backing file in qcow2")

	fmt.Println(" - Check that backing file exists")
	backingFile := filepath.Join(filepath.Dir(diskImage), info.BackingFile)
	_, err = os.Lstat(backingFile)
	nilOrPanic(err, "backingFile missing")

	fmt.Println(" - Garbage collect and test that image is still there")
	nilOrPanic(gc.CollectAll(), "gc.CollectAll() failed")
	_, err = os.Lstat(backingFile)
	nilOrPanic(err, "backingFile missing after GC")
	info = inspectImageFile(diskImage, imageQCOW2Format)
	assert(info != nil, "diskImage for instance deleted after GC")

	fmt.Println(" - Make a new instance")
	instance2, err := manager.Instance("url:test-image-1", func(target string) error {
		panic("We shouldn't get here, as it is currently in the cache")
	})
	nilOrPanic(err, "Failed to create new instance")
	diskImage2 := instance2.DiskFile()
	assert(diskImage2 != diskImage, "Expected a new disk image")
	info = inspectImageFile(diskImage2, imageQCOW2Format)
	assert(info != nil, "diskImage2 missing initially")

	fmt.Println(" - Release the first instance")
	instance.Release()
	_, err = os.Lstat(diskImage)
	assert(os.IsNotExist(err), "first instance diskImage shouldn't exist!")
	info = inspectImageFile(diskImage2, imageQCOW2Format)
	assert(info != nil, "diskImage2 missing after first instance release")

	fmt.Println(" - Garbage collect and test that image is still there")
	nilOrPanic(gc.CollectAll(), "gc.CollectAll() failed")
	_, err = os.Lstat(backingFile)
	nilOrPanic(err, "backingFile missing after second GC")
	_, err = os.Lstat(diskImage)
	assert(os.IsNotExist(err), "first instance diskImage shouldn't exist!")
	info = inspectImageFile(diskImage2, imageQCOW2Format)
	assert(info != nil, "diskImage2 missing after first instance release")

	fmt.Println(" - Release the second instance")
	instance2.Release()
	_, err = os.Lstat(diskImage2)
	assert(os.IsNotExist(err), "second instance diskImage shouldn't exist!")
	_, err = os.Lstat(backingFile)
	nilOrPanic(err, "backingFile missing after release, this shouldn't be...")

	fmt.Println(" - Garbage collect everything") // this should dispose the image
	nilOrPanic(gc.CollectAll(), "gc.CollectAll() failed")
	_, err = os.Lstat(backingFile)
	assert(os.IsNotExist(err), "Expected backingFile to be deleted after GC, file: ", backingFile)

	fmt.Println(" - Check that we can indeed reload the image")
	_, err = manager.Instance("url:test-image-1", func(target string) error {
		return downloadError
	})
	assert(err == downloadError, "Expected a downloadError", err)
}
