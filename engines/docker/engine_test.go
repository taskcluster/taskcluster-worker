package dockerengine

import (
	"os"
	"testing"

	"time"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

// Image and tag used in test cases below
const (
	dockerImageRepository = "alpine"
	dockerImageTag        = "3.6"
)

var provider = &enginetest.EngineProvider{
	Engine: "docker",
	Config: `{}`,
}

func TestMain(m *testing.M) {
	provider.SetupEngine()
	result := 1
	func() {
		defer provider.TearDownEngine()
		result = m.Run()
	}()
	os.Exit(result)
}

func logTime(t *testing.T, name string, f func()) {
	start := time.Now()
	f()
	t.Log(name, ": ", time.Since(start))
}

func TestLogging(t *testing.T) {
	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		TargetPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && true"],
			"image": {
				"repository": "` + dockerImageRepository + `",
				"tag": "` + dockerImageTag + `"
			}
		}`,
		FailingPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && false"],
			"image": {
				"repository": "` + dockerImageRepository + `",
				"tag": "` + dockerImageTag + `"
			}
		}`,
		SilentPayload: `{
			"command": ["sh", "-c", "echo 'no hello' && true"],
			"image": {
				"repository": "` + dockerImageRepository + `",
				"tag": "` + dockerImageTag + `"
			}
		}`,
	}

	// c.Test()
	logTime(t, "TestLogTarget", c.TestLogTarget)
	logTime(t, "TestLogTargetWhenFailing", c.TestLogTargetWhenFailing)
	logTime(t, "TestSilentTask", c.TestSilentTask)
}

func TestKill(t *testing.T) {
	c := enginetest.KillTestCase{
		EngineProvider: provider,
		Target:         `hello-world`,
		Payload: `{
			"command": ["sh", "-c", "echo 'hello-world' && sleep 30 && true"],
			"image": {
				"repository": "` + dockerImageRepository + `",
				"tag": "` + dockerImageTag + `"
			}
		}`,
	}

	logTime(t, "TestKill", c.Test)
	// c.Test()
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
				"repository": "` + dockerImageRepository + `",
				"tag": "` + dockerImageTag + `"
			}
		}`,
	}

	logTime(t, "TestPrintVariable", c.TestPrintVariable)
	// c.TestPrintVariable()
	logTime(t, "TestVariableNameConflict", c.TestVariableNameConflict)
	// c.TestVariableNameConflict()
	logTime(t, "TestInvalidVariableNames", c.TestInvalidVariableNames)
	// c.TestInvalidVariableNames()
	// c.Test()
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
				"repository": "` + dockerImageRepository + `",
				"tag": "` + dockerImageTag + `"
			}
		}`,
	}

	logTime(t, "TestExtractTextFile", c.TestExtractTextFile)
	// c.TestExtractTextFile()
	logTime(t, "TestExtractFileNotFound", c.TestExtractFileNotFound)
	// c.TestExtractFileNotFound()
	logTime(t, "TestExtractFolderNotFound", c.TestExtractFolderNotFound)
	// c.TestExtractFolderNotFound()
	logTime(t, "TestExtractNestedFolderPath", c.TestExtractNestedFolderPath)
	// c.TestExtractNestedFolderPath()
	logTime(t, "TestExtractFolderHandlerInterrupt", c.TestExtractFolderHandlerInterrupt)
	// c.TestExtractFolderHandlerInterrupt()
	// c.Test()
}
