// +build qemu

package qemuengine

import (
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

const testImageFile = "./test-image/tinycore-worker.tar.zst"

// makeTestServer will setup a httptest.Server instance serving the
// testImageFile from the source tree. This is necessary to use the test image
// in our test cases.
func makeTestServer() *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		f, err := os.Open(testImageFile)
		if err != nil {
			log.Panic("Unexpected error opening image file, err: ", err)
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		if err != nil && err != io.EOF {
			log.Panic("Unexpected error copying image file, err: ", err)
		}
	})
	return httptest.NewServer(handler)
}

var s *httptest.Server

func TestMain(m *testing.M) {
	flag.Parse()
	s = makeTestServer()
	provider.SetupEngine()
	result := 1
	func() {
		defer func() {
			provider.TearDownEngine()
			s.CloseClientConnections()
			s.Close()
		}()
		result = m.Run()
	}()
	os.Exit(result)
}

var provider = &enginetest.EngineProvider{
	Engine: "qemu",
	Config: `{
		"maxConcurrency":   5,
		"imageFolder":      "/tmp/images/",
		"socketFolder":     "/tmp/",
		"machineOptions": {
			"maxMemory": 512
		}
  }`,
}

func TestLogging(t *testing.T) {

	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		TargetPayload: `{
      "image": "` + s.URL + `",
      "command": ["sh", "-c", "echo 'hello-world' && true"]
	  }`,
		FailingPayload: `{
	    "image": "` + s.URL + `",
	    "command": ["sh", "-c", "echo 'hello-world' && false"]
	  }`,
		SilentPayload: `{
	    "image": "` + s.URL + `",
	    "command": ["sh", "-c", "echo 'no hello' && true"]
	  }`,
	}

	c.TestLogTarget()
	c.TestLogTargetWhenFailing()
	c.TestSilentTask()
	c.Test()
}

func TestEnvironmentVariables(t *testing.T) {

	c := enginetest.EnvVarTestCase{
		EngineProvider: provider,
		VariableName:   "TEST_ENV_VAR",
		InvalidVariableNames: []string{
			"#=#",
		},
		Payload: `{
      "image": "` + s.URL + `",
      "command": ["sh", "-c", "echo $TEST_ENV_VAR && true"]
	  }`,
	}

	c.TestPrintVariable()
	c.TestVariableNameConflict()
	c.TestInvalidVariableNames()
	c.Test()
}

func TestAttachProxy(t *testing.T) {

	c := enginetest.ProxyTestCase{
		EngineProvider: provider,
		ProxyName:      "test-proxy",
		PingProxyPayload: `{
			"image": "` + s.URL + `",
			"command": ["sh", "-ec", "echo 'Pinging'; STATUS=$(curl -s -o /tmp/output -w '%{http_code}' http://taskcluster/test-proxy/v1/ping); cat /tmp/output; test $STATUS -eq 200;"]
		}`,
	}

	c.TestPingProxyPayload()
	c.TestPing404IsUnsuccessful()
	c.TestLiveLogging()
	c.TestParallelPings()
	c.Test()
}

func TestArtifacts(t *testing.T) {
	c := enginetest.ArtifactTestCase{
		EngineProvider:     provider,
		Text:               "[hello-world]",
		TextFilePath:       "/home/tc/folder/hello.txt",
		FileNotFoundPath:   "/home/tc/no-such-file.txt",
		FolderNotFoundPath: "/home/tc/no-such-folder/",
		NestedFolderFiles: []string{
			"hello.txt",
			"sub-folder/hello2.txt",
		},
		NestedFolderPath: "/home/tc/folder/",
		Payload: `{
			"image": "` + s.URL + `",
			"command": ["sh", "-ec", "mkdir -p /home/tc/folder/sub-folder; echo '[hello-world]' > /home/tc/folder/hello.txt; echo '[hello-world]' > /home/tc/folder/sub-folder/hello2.txt"]
		}`,
	}

	c.TestExtractTextFile()
	c.TestExtractFileNotFound()
	c.TestExtractFolderNotFound()
	c.TestExtractNestedFolderPath()
	c.TestExtractFolderHandlerInterrupt()
	c.Test()
}

func TestShell(t *testing.T) {
	c := enginetest.ShellTestCase{
		EngineProvider: provider,
		Command:        "echo '[hello-world]'; (>&2 echo '[hello-error]');",
		Stdout:         "[hello-world]\n",
		Stderr:         "[hello-error]\n",
		BadCommand:     "exit 1;\n",
		SleepCommand:   "sleep 30;\n",
		Payload: `{
      "image": "` + s.URL + `",
      "command": ["sh", "-c", "sleep 5 && true"]
	  }`,
	}

	c.TestCommand()
	c.TestBadCommand()
	c.TestAbortSleepCommand()
	c.TestKillSleepCommand()
	c.Test()
}

func TestDisplay(t *testing.T) {
	c := enginetest.DisplayTestCase{
		EngineProvider: provider,
		Displays: []engines.Display{
			{
				Name:        "screen",
				Description: "Primary screen attached to the virtual machine",
				Width:       0,
				Height:      0,
			},
		},
		InvalidDisplayName: "invalid-screen",
		Payload: `{
      "image": "` + s.URL + `",
      "command": ["sh", "-c", "true"]
    }`,
	}

	c.TestListDisplays()
	c.TestDisplays()
	c.TestInvalidDisplayName()
	c.TestKillDisplay()
	c.Test()
}

func TestKill(t *testing.T) {
	c := enginetest.KillTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		Payload: `{
      "image": "` + s.URL + `",
      "command": ["sh", "-c", "echo 'hello-world' && sleep 30 && true"]
    }`,
	}

	c.Test()
}
