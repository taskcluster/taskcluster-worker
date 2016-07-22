package qemuguesttools

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func fmtPanic(a ...interface{}) {
	panic(fmt.Sprintln(a...))
}

func nilOrPanic(err error, a ...interface{}) {
	if err != nil {
		fmtPanic(append(a, err)...)
	}
}

func assert(condition bool, a ...interface{}) {
	if !condition {
		fmtPanic(a...)
	}
}

func TestGuestToolsProcessingActions(t *testing.T) {
	// Create temporary storage
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}
	environment := &runtime.Environment{
		TemporaryStorage: storage,
	}

	logTask := bytes.NewBuffer(nil)
	meta := metaservice.New([]string{}, map[string]string{}, logTask, func(r bool) {
		panic("This test shouldn't get to this point!")
	}, environment)

	// Create http server for testing
	ts := httptest.NewServer(meta)
	defer ts.Close()
	defer meta.StopPollers() // Hack to stop pollers, otherwise server will block
	u, err := url.Parse(ts.URL)
	if err != nil {
		panic("Expected a url we can parse")
	}

	// Create a logger
	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "guest-tools-tests")

	// Create an run guest-tools
	g := new(u.Host, log)

	// start processing actions
	g.StartProcessingActions()
	defer g.StopProcessingActions()

	////////////////////
	debug("### Test meta.GetArtifact")
	f, err := storage.NewFolder()
	if err != nil {
		panic("Failed to create temp folder")
	}
	defer f.Remove()

	testFile := filepath.Join(f.Path(), "hello.txt")
	err = ioutil.WriteFile(testFile, []byte("hello-world"), 0777)
	nilOrPanic(err, "Failed to create testFile: ", testFile)

	debug(" - request file: %s", testFile)
	r, err := meta.GetArtifact(testFile)
	nilOrPanic(err, "meta.GetArtifact failed, error: ", err)

	debug(" - reading testFile")
	data, err := ioutil.ReadAll(r)
	nilOrPanic(err, "Failed to read testFile")
	debug(" - read: '%s'", string(data))
	assert(string(data) == "hello-world", "Wrong payload: ", string(data))

	////////////////////
	debug("### Test meta.GetArtifact (missing file)")
	r, err = meta.GetArtifact(filepath.Join(f.Path(), "missing-file.txt"))
	assert(r == nil, "Expected error wihtout a reader")
	assert(err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound")

	////////////////////
	debug("### Test meta.ListFolder")
	testFolder := filepath.Join(f.Path(), "test-folder")
	err = os.Mkdir(testFolder, 0777)
	nilOrPanic(err, "Failed to create test-folder/")

	testFile2 := filepath.Join(testFolder, "hello2.txt")
	err = ioutil.WriteFile(testFile2, []byte("hello-world-2"), 0777)
	nilOrPanic(err, "Failed to create testFile2: ", testFile2)

	debug(" - meta.ListFolder")
	files, err := meta.ListFolder(f.Path())
	nilOrPanic(err, "ListFolder failed, err: ", err)

	assert(len(files) == 2, "Expected 2 files")
	assert(files[0] == testFile || files[1] == testFile, "Expected testFile")
	assert(files[0] == testFile2 || files[1] == testFile2, "Expected testFile2")

	////////////////////
	debug("### Test meta.ListFolder (missing folder)")
	files, err = meta.ListFolder(filepath.Join(f.Path(), "no-such-folder"))
	assert(files == nil, "Expected files == nil, we hopefully have an error")
	assert(err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound")

	////////////////////
	debug("### Test meta.ListFolder (empty folder)")
	emptyFolder := filepath.Join(f.Path(), "empty-folder")
	err = os.Mkdir(emptyFolder, 0777)
	nilOrPanic(err, "Failed to create empty-folder/")

	files, err = meta.ListFolder(emptyFolder)
	assert(len(files) == 0, "Expected zero files")
	assert(err == nil, "Didn't expect any error")
}
