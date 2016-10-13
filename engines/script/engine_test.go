package scriptengine

import (
	t "testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

var provider = &enginetest.EngineProvider{
	Engine: "script",
	Config: `{
    "command": ["bash", "-ec", "v=$(cat); echo \"$v\"; echo \"$v\" | grep success"],
    "schema": {
      "type": "object",
      "properties": {
        "arg": {"type": "string"}
      },
      "required": ["arg"]
    },
    "expiration": 3
  }`,
}

var loggingTestCase = enginetest.LoggingTestCase{
	EngineProvider: provider,
	Target:         "hello-world",
	TargetPayload: `{
    "arg": "hello-world, this is a successful task"
  }`,
	FailingPayload: `{
    "arg": "hello-world, this is a failing task"
  }`,
	SilentPayload: `{
    "arg": "This is a successful task, that doesn't log target string"
  }`,
}

func TestLogTarget(t *t.T)            { loggingTestCase.TestLogTarget() }
func TestLogTargetWhenFailing(t *t.T) { loggingTestCase.TestLogTargetWhenFailing() }
func TestSilentTask(t *t.T)           { loggingTestCase.TestSilentTask() }
func TestLoggingTestCase(t *t.T)      { loggingTestCase.Test() }
