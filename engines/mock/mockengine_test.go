package mockengine

import (
	"encoding/json"
	t "testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

func parseTestPayload(t *t.T, engine engines.Engine, payload string) interface{} {
	jsonPayload := map[string]json.RawMessage{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	if err != nil {
		t.Fatal("Test payload parsing failed: ", err, " payload: ", payload)
	}
	p, err := engine.PayloadSchema().Parse(jsonPayload)
	if err != nil {
		t.Fatal("Test payload validation failed: ", err, " payload: ", payload)
	}
	return p
}

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

func TestVolumeTestCase(t *t.T) { volumeTestCase.Test(t) }

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

func TestLoggingTestCase(t *t.T) { loggingTestCase.Test(t) }

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
