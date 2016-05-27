// +build darwin

package osxnative

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	assert "github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"io/ioutil"
	"os"
	osuser "os/user"
	"path"
	"testing"
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

	engine := engine{
		EngineBase: engines.EngineBase{},
		log:        logrus.New().WithField("component", "test"),
	}

	return resultset{
		ResultSetBase: engines.ResultSetBase{},
		taskUser:      user{},
		context:       context,
		success:       true,
		engine:        &engine,
	}
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

	userInfo, err := osuser.Current()
	assert.NoError(t, err)

	r := makeResultSet(t)

	for _, tc := range testCases {
		if r.validPath(userInfo.HomeDir, tc.pathName) != tc.expectedResult {
			t.Errorf("validPath(%s) != %t", tc.pathName, tc.expectedResult)
		}
	}
}

func TestExtractFile(t *testing.T) {
	r := makeResultSet(t)
	defer r.Dispose()

	_, err := r.ExtractFile("invalid-path/invalid-file")
	assert.Equal(t, err, engines.ErrResourceNotFound)

	file, err := r.ExtractFile("test-data/test.txt")
	assert.NoError(t, err)

	data, err := ioutil.ReadAll(file)

	assert.NoError(t, err)
	assert.Equal(t, string(data), "test.txt\n")
	assert.NoError(t, file.Close())
}

func TestExtractFolder(t *testing.T) {
	r := makeResultSet(t)
	defer r.Dispose()

	err := r.ExtractFolder("invalid-path/", func(p string, stream ioext.ReadSeekCloser) error {
		return nil
	})

	assert.Equal(t, err, engines.ErrResourceNotFound)

	err = r.ExtractFolder("test-data", func(p string, stream ioext.ReadSeekCloser) error {
		expected := path.Base(p) + "\n"
		data, err := ioutil.ReadAll(stream)
		sdata := string(data)

		if err != nil {
			return err
		}

		if sdata != expected {
			return fmt.Errorf(
				"Invalid file contents. content: \"%s\" expected: \"%s\"",
				sdata,
				expected,
			)
		}

		return nil
	})

	assert.NoError(t, err)
}
