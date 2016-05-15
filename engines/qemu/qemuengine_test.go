// +build disabled

// Change the build tag to "vagrant"

package qemuengine

import "github.com/taskcluster/taskcluster-worker/engines/enginetest"

var provider = enginetest.EngineProvider{
	Engine: "qemu",
	Config: `{
    "maxConcurrency":   2,
    "imageFolder":      "/tmp/images/",
    "socketFolder":     "/tmp/"
  }`,
}

// TODO: Setup server hosting image on localhost

var loggingTestCase = enginetest.LoggingTestCase{
	EngineProvider: provider,
	Target:         "Hello World",
	TargetPayload: `{
    "start": {
      "image": "http://...",
      "command": ["sh", "-c", "echo 'hello world' && true"]
    }
  }`,
	FailingPayload: `{
    "start": {
    "image": "http://...",
    "command": ["sh", "-c", "echo 'hello world' && false"]
    }
  }`,
	SilentPayload: `{
    "start": {
    "image": "http://...",
    "command": ["sh", "-c", "echo 'no hello' && true"]
    }
  }`,
}

func TestLogTarget(t *t.T) { loggingTestCase.TestLogTarget() }

//func TestLogTargetWhenFailing(t *t.T) { loggingTestCase.TestLogTargetWhenFailing() }
//func TestSilentTask(t *t.T)           { loggingTestCase.TestSilentTask() }
//func TestLoggingTestCase(t *t.T)      { loggingTestCase.Test() }
