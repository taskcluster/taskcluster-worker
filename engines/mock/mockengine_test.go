package mockengine

import (
	t "testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

var volumeTestCase = enginetest.VolumeTestCase{
	Engine:     "mock",
	Mountpoint: "/mock/volume",
	WriteVolumePayload: `{
    "start": {
      "delay": 10,
      "function": "set-volume",
      "argument": "/mock/volume"
    }
  }`,
	CheckVolumePayload: `{
    "start": {
      "delay": 10,
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
	Engine: "mock",
	Target: "Hello World",
	TargetPayload: `{
    "start": {
      "delay": 10,
      "function": "write-log",
      "argument": "Hello World"
    }
  }`,
	FailingPayload: `{
    "start": {
      "delay": 10,
      "function": "write-error-log",
      "argument": "Hello World"
    }
  }`,
	SilentPayload: `{
    "start": {
      "delay": 10,
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
	Engine:    "mock",
	ProxyName: "proxy.com",
	PingProxyPayload: `{
    "start": {
      "delay": 10,
      "function": "ping-proxy",
      "argument": "http://proxy.com/v1/ping"
    }
  }`,
}

func TestPingProxyPayload(t *t.T)      { proxyTestCase.TestPingProxyPayload() }
func TestPing404IsUnsuccessful(t *t.T) { proxyTestCase.TestPing404IsUnsuccessful() }
func TestLiveLogging(t *t.T)           { proxyTestCase.TestLiveLogging() }
func TestParallelPings(t *t.T)         { proxyTestCase.TestParallelPings() }
func TestProxyTestCase(t *t.T)         { proxyTestCase.Test(t) }

var envVarTestCase = enginetest.EnvVarTestCase{
	Engine:               "mock",
	VariableName:         "HELLO_WORLD",
	InvalidVariableNames: []string{"bad d", "also bad", "can't have space"},
	Payload: `{
    "start": {
      "delay": 10,
      "function": "print-env-var",
      "argument": "HELLO_WORLD"
    }
  }`,
}

func TestPrintVariable(t *t.T)        { envVarTestCase.TestPrintVariable() }
func TestVariableNameConflict(t *t.T) { envVarTestCase.TestVariableNameConflict() }
func TestInvalidVariableNames(t *t.T) { envVarTestCase.TestInvalidVariableNames() }
func TestEnvVarTestCase(t *t.T)       { envVarTestCase.Test() }
