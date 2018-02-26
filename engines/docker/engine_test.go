package dockerengine

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

var provider = &enginetest.EngineProvider{
	Engine: "docker",
	Config: `{
		"dockerEndpoint": "unix:///var/run/docker.sock",
		"maxConcurrency": 1
	}`,
}

func TestLogging(t *testing.T) {
	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		TargetPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && true"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
		FailingPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && false"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
		SilentPayload: `{
			"command": ["sh", "-c", "echo 'no hello' && true"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
	}

	c.Test()
}

func TestKill(t *testing.T) {
	c := enginetest.KillTestCase{
		EngineProvider: provider,
		Target:         `hello-world`,
		Payload: `{
			"command": ["sh", "-c", "echo 'hello-world' && sleep 30 && true"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
	}

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
			"command": ["sh", "-c", "echo $TEST_ENV_VAR && true"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
	}

	c.TestPrintVariable()
	c.TestVariableNameConflict()
	c.TestInvalidVariableNames()
	c.Test()
}

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
			"command": ["sh", "-ec", "mkdir -p folder/sub-folder; echo '[hello-world]' > folder/hello.txt; echo '[hello-world]' > folder/sub-folder/hello2.txt"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
	}

	// c.TestExtractTextFile()
	// c.TestExtractFileNotFound()
	// c.TestExtractFolderNotFound()
	// c.TestExtractNestedFolderPath()
	// c.TestExtractFolderHandlerInterrupt()
	c.Test()
}
