package mockengine

import (
	"encoding/json"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

func parseTestPayload(t *testing.T, engine engines.Engine, payload string) interface{} {
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

func TestVolumeTestCase(t *testing.T) { t.Parallel(); volumeTestCase.Test(t) }
