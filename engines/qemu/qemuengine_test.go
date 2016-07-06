// +build qemu

package qemuengine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

const testImageFile = "./image/tinycore-worker.tar.lz4"

// makeTestServer will setup a httptest.Server instance serving the
// testImageFile from the source tree. This is necessary to use the test image
// in our test cases.
func makeTestServer() *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		f, err := os.Open(testImageFile)
		if err != nil {
			fmtPanic("Unexpected error opening image file, err: ", err)
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		if err != nil && err != io.EOF {
			fmtPanic("Unexpected error copying image file, err: ", err)
		}
	})
	return httptest.NewServer(handler)
}

var provider = enginetest.EngineProvider{
	Engine: "qemu",
	Config: `{
		"qemu": {
			"maxConcurrency":   2,
			"imageFolder":      "/tmp/images/",
			"socketFolder":     "/tmp/"
		}
  }`,
}

func TestLogTarget(t *testing.T) {
	s := makeTestServer()
	defer func() {
		s.CloseClientConnections()
		s.Close()
	}()

	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "Hello World",
		TargetPayload: `{
	    "start": {
	      "image": "` + s.URL + `",
	      "command": ["sh", "-c", "echo 'Hello World' && true"]
	    }
	  }`,
		FailingPayload: `{
	    "start": {
		    "image": "` + s.URL + `",
		    "command": ["sh", "-c", "echo 'hello world' && false"]
	    }
	  }`,
		SilentPayload: `{
	    "start": {
		    "image": "` + s.URL + `",
		    "command": ["sh", "-c", "echo 'no hello' && true"]
	    }
	  }`,
	}

	c.TestLogTarget()
}

//func TestLogTargetWhenFailing(t *t.T) { loggingTestCase.TestLogTargetWhenFailing() }
//func TestSilentTask(t *t.T)           { loggingTestCase.TestSilentTask() }
//func TestLoggingTestCase(t *t.T)      { loggingTestCase.Test() }
