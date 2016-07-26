package qemuguesttools

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
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

	////////////////////
	testShellHello(meta)
	testShellCat(meta)
	testShellCatStdErr(meta)
}

func testShellHello(meta *metaservice.MetaService) {
	debug("### Test meta.Shell (using 'echo hello')")
	shell, err := meta.ExecShell()
	nilOrPanic(err, "Failed to call meta.ExecShell()")

	readHello := sync.WaitGroup{}
	readHello.Add(1)
	// Discard stderr
	go io.Copy(ioutil.Discard, shell.StderrPipe())
	go func() {
		shell.StdinPipe().Write([]byte("echo HELLO\n"))
		readHello.Wait()
		shell.StdinPipe().Close()
	}()
	go func() {
		data := bytes.Buffer{}
		for {
			b := []byte{0}
			n, werr := shell.StdoutPipe().Read(b)
			data.Write(b[:n])
			if strings.Contains(data.String(), "HELLO") {
				readHello.Done()
				break
			}
			if werr != nil {
				assert(werr == io.EOF, "Expected EOF!")
				break
			}
		}
		// Discard the rest
		go io.Copy(ioutil.Discard, shell.StdoutPipe())
	}()

	success, err := shell.Wait()
	nilOrPanic(err, "Got an error from shell.Wait, error: ", err)
	assert(success, "Expected success from shell, we closed with end of stdin")
}

func testShellCat(meta *metaservice.MetaService) {
	debug("### Test meta.Shell (using 'exec cat -')")
	shell, err := meta.ExecShell()
	nilOrPanic(err, "Failed to call meta.ExecShell()")

	input := make([]byte, 42*1024*1024+7)
	rand.Read(input)

	// Discard stderr
	go io.Copy(ioutil.Discard, shell.StderrPipe())
	go func() {
		if goruntime.GOOS == "windows" {
			shell.StdinPipe().Write([]byte("type con\n"))
		} else {
			shell.StdinPipe().Write([]byte("exec cat -\n"))
		}
		shell.StdinPipe().Write(input)
		shell.StdinPipe().Close()
		debug("Closed stdin")
	}()
	var output []byte
	outputDone := sync.WaitGroup{}
	outputDone.Add(1)
	go func() {
		data, rerr := ioutil.ReadAll(shell.StdoutPipe())
		nilOrPanic(rerr, "Got error from stdout pipe, error: ", rerr)
		output = data
		outputDone.Done()
	}()

	success, err := shell.Wait()
	nilOrPanic(err, "Got an error from shell.Wait, error: ", err)
	assert(success, "Expected success from shell, we closed with end of stdin")
	outputDone.Wait()
	assert(bytes.Compare(output, input) == 0, "Expected data to match input")
}

func testShellCatStdErr(meta *metaservice.MetaService) {
	debug("### Test meta.Shell (using 'exec cat - 1>&2')")
	shell, err := meta.ExecShell()
	nilOrPanic(err, "Failed to call meta.ExecShell()")

	input := make([]byte, 4*1024*1024+37)
	rand.Read(input)

	// Discard stderr
	go io.Copy(ioutil.Discard, shell.StdoutPipe())
	go func() {
		if goruntime.GOOS == "windows" {
			shell.StdinPipe().Write([]byte("type con 1>&2\n"))
		} else {
			shell.StdinPipe().Write([]byte("exec cat -  1>&2\n"))
		}
		shell.StdinPipe().Write(input)
		shell.StdinPipe().Close()
		debug("Closed stdin")
	}()
	var output []byte
	outputDone := sync.WaitGroup{}
	outputDone.Add(1)
	go func() {
		data, rerr := ioutil.ReadAll(shell.StderrPipe())
		nilOrPanic(rerr, "Got error from stderr pipe, error: ", rerr)
		output = data
		outputDone.Done()
	}()

	success, err := shell.Wait()
	nilOrPanic(err, "Got an error from shell.Wait, error: ", err)
	assert(success, "Expected success from shell, we closed with end of stdin")
	outputDone.Wait()
	assert(bytes.Compare(output, input) == 0, "Expected data to match input")
}
