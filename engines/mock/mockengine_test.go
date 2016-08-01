package mockengine

import (
	t "testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

var provider = enginetest.EngineProvider{
	Engine: "mock",
	Config: "{}",
}

var volumeTestCase = enginetest.VolumeTestCase{
	EngineProvider: provider,
	Mountpoint:     "/mock/volume",
	WriteVolumePayload: `{
    "start": {
      "delay": 0,
      "function": "set-volume",
      "argument": "/mock/volume"
    }
  }`,
	CheckVolumePayload: `{
    "start": {
      "delay": 0,
      "function": "get-volume",
      "argument": "/mock/volume"
    }
  }`,
}

func TestWriteReadVolume(*t.T)       { volumeTestCase.TestWriteReadVolume() }
func TestReadEmptyVolume(*t.T)       { volumeTestCase.TestReadEmptyVolume() }
func TestWriteToReadOnlyVolume(*t.T) { volumeTestCase.TestWriteToReadOnlyVolume() }
func TestReadToReadOnlyVolume(*t.T)  { volumeTestCase.TestReadToReadOnlyVolume() }
func TestVolumeTestCase(t *t.T)      { volumeTestCase.Test() }

var loggingTestCase = enginetest.LoggingTestCase{
	EngineProvider: provider,
	Target:         "Hello World",
	TargetPayload: `{
    "start": {
      "delay": 0,
      "function": "write-log",
      "argument": "Hello World"
    }
  }`,
	FailingPayload: `{
    "start": {
      "delay": 0,
      "function": "write-error-log",
      "argument": "Hello World"
    }
  }`,
	SilentPayload: `{
    "start": {
      "delay": 0,
      "function": "write-log",
      "argument": "Okay, let's try on Danish then: 'Hej Verden'"
    }
  }`,
}

func TestLogTarget(t *t.T)            { loggingTestCase.TestLogTarget() }
func TestLogTargetWhenFailing(t *t.T) { loggingTestCase.TestLogTargetWhenFailing() }
func TestSilentTask(t *t.T)           { loggingTestCase.TestSilentTask() }
func TestLoggingTestCase(t *t.T)      { loggingTestCase.Test() }

var proxyTestCase = enginetest.ProxyTestCase{
	EngineProvider: provider,
	ProxyName:      "proxy.com",
	PingProxyPayload: `{
    "start": {
      "delay": 0,
      "function": "ping-proxy",
      "argument": "http://proxy.com/v1/ping"
    }
  }`,
}

func TestPingProxyPayload(t *t.T)      { proxyTestCase.TestPingProxyPayload() }
func TestPing404IsUnsuccessful(t *t.T) { proxyTestCase.TestPing404IsUnsuccessful() }
func TestLiveLogging(t *t.T)           { proxyTestCase.TestLiveLogging() }
func TestParallelPings(t *t.T)         { proxyTestCase.TestParallelPings() }
func TestProxyTestCase(t *t.T)         { proxyTestCase.Test() }

var envVarTestCase = enginetest.EnvVarTestCase{
	EngineProvider:       provider,
	VariableName:         "HELLO_WORLD",
	InvalidVariableNames: []string{"bad d", "also bad", "can't have space"},
	Payload: `{
    "start": {
      "delay": 0,
      "function": "print-env-var",
      "argument": "HELLO_WORLD"
    }
  }`,
}

func TestPrintVariable(t *t.T)        { envVarTestCase.TestPrintVariable() }
func TestVariableNameConflict(t *t.T) { envVarTestCase.TestVariableNameConflict() }
func TestInvalidVariableNames(t *t.T) { envVarTestCase.TestInvalidVariableNames() }
func TestEnvVarTestCase(t *t.T)       { envVarTestCase.Test() }

var artifactTestCase = enginetest.ArtifactTestCase{
	EngineProvider:     provider,
	Text:               "Hello World",
	TextFilePath:       "/folder/a.txt",
	FileNotFoundPath:   "/not-found.txt",
	FolderNotFoundPath: "/no-folder/",
	NestedFolderFiles:  []string{"/folder/a.txt", "/folder/b.txt", "/folder/c/c.txt"},
	NestedFolderPath:   "/folder/",
	Payload: `{
		"start":{
			"delay": 0,
			"function": "write-files",
			"argument": "/folder/a.txt /folder/b.txt /folder/c/c.txt"
		}
	}`,
}

func TestExtractTextFile(t *t.T)               { artifactTestCase.TestExtractTextFile() }
func TestExtractFileNotFound(t *t.T)           { artifactTestCase.TestExtractFileNotFound() }
func TestExtractFolderNotFound(t *t.T)         { artifactTestCase.TestExtractFolderNotFound() }
func TestExtractNestedFolderPath(t *t.T)       { artifactTestCase.TestExtractNestedFolderPath() }
func TestExtractFolderHandlerInterrupt(t *t.T) { artifactTestCase.TestExtractFolderHandlerInterrupt() }
func TestArtifactTestCase(t *t.T)              { artifactTestCase.Test() }

var shellTestCase = enginetest.ShellTestCase{
	EngineProvider: provider,
	Command:        "print-hello",
	Stdout:         "Hello World",
	Stderr:         "No error!",
	BadCommand:     "exit-false",
	SleepCommand:   "sleep",
	Payload: `{
		"start":{
			"delay": 0,
			"function": "true",
			"argument": ""
		}
	}`,
}

func TestCommand(t *t.T)           { shellTestCase.TestCommand() }
func TestBadCommand(t *t.T)        { shellTestCase.TestBadCommand() }
func TestAbortSleepCommand(t *t.T) { shellTestCase.TestAbortSleepCommand() }
func Test(t *t.T)                  { shellTestCase.Test() }

var displayTestCase = enginetest.DisplayTestCase{
	EngineProvider: provider,
	Displays: []engines.Display{
		{
			Name:        "MockDisplay",
			Description: "Simple mock VNC display rendering a static test image",
			Width:       mockDisplayWidth,
			Height:      mockDisplayHeight,
		},
	},
	InvalidDisplayName: "no-such-display",
	Payload: `{
		"start":{
			"delay": 0,
			"function": "true",
			"argument": ""
		}
	}`,
}

func TestListDisplays(t *t.T)       { displayTestCase.TestListDisplays() }
func TestDisplays(t *t.T)           { displayTestCase.TestDisplays() }
func TestInvalidDisplayName(t *t.T) { displayTestCase.TestInvalidDisplayName() }
func TestDisplayTestCase(t *t.T)    { displayTestCase.Test() }
