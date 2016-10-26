// +build linux,native

package nativeengine

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

var provider = &enginetest.EngineProvider{
	Engine: "native",
	Config: `{}`,
}

func TestLogging(t *testing.T) {
	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		TargetPayload: `{
      "command": ["sh", "-c", "echo 'hello-world' && true"]
	  }`,
		FailingPayload: `{
	    "command": ["sh", "-c", "echo 'hello-world' && false"]
	  }`,
		SilentPayload: `{
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
      "command": ["sh", "-c", "echo $TEST_ENV_VAR && true"]
	  }`,
	}

	c.TestPrintVariable()
	c.TestVariableNameConflict()
	c.TestInvalidVariableNames()
	c.Test()
}

/*

func TestAttachProxy(t *testing.T) {

	c := enginetest.ProxyTestCase{
		EngineProvider: provider,
		ProxyName:      "test-proxy",
		PingProxyPayload: `{
			"command": ["sh", "-ec", "echo 'Pinging'; STATUS=$(curl -s -o /tmp/output -w '%{http_code}' http://taskcluster/test-proxy/v1/ping); cat /tmp/output; test $STATUS -eq 200;"]
		}`,
	}

	c.TestPingProxyPayload()
	c.TestPing404IsUnsuccessful()
	c.TestLiveLogging()
	c.TestParallelPings()
	c.Test()
}
*/

func TestArtifacts(t *testing.T) {
	c := enginetest.ArtifactTestCase{
		EngineProvider:     provider,
		Text:               "[hello-world]",
		TextFilePath:       "folder/hello.txt",
		FileNotFoundPath:   "no-such-file.txt",
		FolderNotFoundPath: "no-such-folder/",
		NestedFolderFiles: []string{
			"hello.txt",
			"sub-folder/hello2.txt",
		},
		NestedFolderPath: "folder/",
		Payload: `{
			"command": ["sh", "-ec", "mkdir -p folder/sub-folder; echo '[hello-world]' > folder/hello.txt; echo '[hello-world]' > folder/sub-folder/hello2.txt"]
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
      "command": ["sh", "-c", "true"]
	  }`,
	}

	c.TestCommand()
	c.TestBadCommand()
	c.TestAbortSleepCommand()
	c.Test()
}
