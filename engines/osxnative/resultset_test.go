// +build darwin

package osxnative

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type testCase struct {
	pathName       string
	expectedResult bool
}

func makeResultSet(t *testing.T) resultset {
	temp, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	context, _, err := runtime.NewTaskContext(temp.NewFilePath(), runtime.TaskInfo{})
	if err != nil {
		t.Fatal(err)
	}

	return newResultSet(context, true)
}

func TestValidPath(t *testing.T) {
	home := os.Getenv("HOME")

	testCases := []testCase{
		{path.Join(home, "test"), true},
		{path.Join(home, "dir", "test"), true},
		{home, true},
		{path.Join(home, "."), true},
		{"/tmp", true},
		{"/tmp/test", true},
		{"/tmp/dir/test", true},
		{"/tmp/../tmp", true},
		{"/usr", false},
		{path.Dir(home), false},
		{path.Join(home, ".."), false},
		{"/", false},
	}

	r := makeResultSet(t)

	for _, tc := range testCases {
		if r.validPath(tc.pathName) != tc.expectedResult {
			t.Errorf("validPath(%s) != %t", tc.pathName, tc.expectedResult)
		}
	}
}

func TestExtractFile(t *testing.T) {
	r := makeResultSet(t)

	_, err := r.ExtractFile("invalid-path/invalid-file")

	if err != engines.ErrResourceNotFound {
		t.Fatalf("Invalid error type %v\n", err)
	}

	file, err := r.ExtractFile("test-data/test.txt")

	if err != nil {
		t.Fatalf("File test/test.txt returned failure: %v\n", err)
	}

	data, err := ioutil.ReadAll(file)

	if err != nil {
		t.Fatalf("Error reading file: %v\n", err)
	}

	sdata := string(data)
	if sdata != "test.txt\n" {
		t.Fatalf("File content doesn't match \"%s\"\n", sdata)
	}

	err = file.Close()

	if err != nil {
		t.Fatal(err)
	}
}

func TestExtractFolder(t *testing.T) {
	r := makeResultSet(t)

	err := r.ExtractFolder("invalid-path/", func(p string, stream ioext.ReadSeekCloser) error {
		return nil
	})

	if err != engines.ErrResourceNotFound {
		t.Fatalf("Invalid error type %v\n", err)
	}

	err = r.ExtractFolder("test-data", func(p string, stream ioext.ReadSeekCloser) error {
		expected := path.Base(p) + "\n"
		data, err := ioutil.ReadAll(stream)
		sdata := string(data)

		if err != nil {
			return err
		}

		if sdata != expected {
			return errors.New(fmt.Sprintf(
				"Invalid file contents. content: \"%s\" expected: \"%s\"",
				sdata,
				expected,
			))
		}

		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
}
