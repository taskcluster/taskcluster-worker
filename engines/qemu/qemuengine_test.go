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
			"maxConcurrency":   6,
			"imageFolder":      "/tmp/images/",
			"socketFolder":     "/tmp/"
		}
  }`,
}

func TestLogging(t *testing.T) {
	s := makeTestServer()
	defer func() {
		s.CloseClientConnections()
		s.Close()
	}()

	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		TargetPayload: `{
	    "start": {
	      "image": "` + s.URL + `",
	      "command": ["sh", "-c", "echo 'hello-world' && true"]
	    }
	  }`,
		FailingPayload: `{
	    "start": {
		    "image": "` + s.URL + `",
		    "command": ["sh", "-c", "echo 'hello-world' && false"]
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
	c.TestLogTargetWhenFailing()
	c.TestSilentTask()
	c.Test()
}

func TestEnvironmentVariables(t *testing.T) {
	s := makeTestServer()
	defer func() {
		s.CloseClientConnections()
		s.Close()
	}()

	c := enginetest.EnvVarTestCase{
		EngineProvider: provider,
		VariableName:   "TEST_ENV_VAR",
		InvalidVariableNames: []string{
			"#=#",
		},
		Payload: `{
	    "start": {
	      "image": "` + s.URL + `",
	      "command": ["sh", "-c", "echo $TEST_ENV_VAR && true"]
	    }
	  }`,
	}

	c.TestPrintVariable()
	c.TestVariableNameConflict()
	c.TestInvalidVariableNames()
	c.Test()
}

func TestAttachProxy(t *testing.T) {
	s := makeTestServer()
	defer func() {
		s.CloseClientConnections()
		s.Close()
	}()

	c := enginetest.ProxyTestCase{
		EngineProvider: provider,
		ProxyName:      "test-proxy",
		PingProxyPayload: `{
			"start": {
				"image": "` + s.URL + `",
				"command": ["sh", "-ec", "echo 'Pinging'; STATUS=$(curl -s -o /tmp/output -w '%{http_code}' http://taskcluster/test-proxy/v1/ping); cat /tmp/output; test $STATUS -eq 200;"]
			}
		}`,
	}

	c.TestPingProxyPayload()
	c.TestPing404IsUnsuccessful()
	c.TestLiveLogging()
	c.TestParallelPings()
	c.Test()
}
