package reboot

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestRebootDoesNothing(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: false,
		PluginConfig:      `{}`,
		Payload: `{
			"delay": 10,
			"function": "true",
			"argument": ""
		}`,
	}.Test()
}

func TestRebootTaskLimitOne(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"taskLimit": 1
		}`,
		Payload: `{
			"delay": 10,
			"function": "true",
			"argument": ""
		}`,
	}.Test()
}

func TestRebootTaskLimitTwo(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: false,
		PluginConfig: `{
			"taskLimit": 2
		}`,
		Payload: `{
			"delay": 10,
			"function": "true",
			"argument": ""
		}`,
	}.Test()
}

func TestRebootMaxLifeCycle(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"maxLifeCycle": 1
		}`,
		Payload: `{
			"delay": 1500,
			"function": "true",
			"argument": ""
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsNothing(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: false,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "true",
			"argument": ""
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsAlwaysSuccess(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "true",
			"argument": "",
			"reboot": "always"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsAlwaysFailure(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     false,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "false",
			"argument": "",
			"reboot": "always"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsAlwaysMalformedPayload(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     false,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "malformed-payload-initial",
			"argument": "bad-payload-who-knows",
			"reboot": "always"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsOnFailureSuccess(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: false,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "true",
			"argument": "",
			"reboot": "on-failure"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsOnFailureFail(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     false,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "false",
			"argument": "",
			"reboot": "on-failure"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsOnFailureMalformedPayload(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     false,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "malformed-payload-initial",
			"argument": "bad-payload-who-knows",
			"reboot": "on-failure"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsOnExceptionSuccess(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     true,
		PropagateSuccess:  true,
		StoppedGracefully: false,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "true",
			"argument": "",
			"reboot": "on-exception"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsOnExceptionFail(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     false,
		PropagateSuccess:  true,
		StoppedGracefully: false,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "false",
			"argument": "",
			"reboot": "on-exception"
		}`,
	}.Test()
}

func TestRebootAllowTaskRebootsOnExceptionMalformedPayload(t *testing.T) {
	plugintest.Case{
		Plugin:            "reboot",
		PluginSuccess:     true,
		EngineSuccess:     false,
		PropagateSuccess:  true,
		StoppedGracefully: true,
		PluginConfig: `{
			"allowTaskReboots": true
		}`,
		Payload: `{
			"delay": 10,
			"function": "malformed-payload-initial",
			"argument": "bad-payload-who-knows",
			"reboot": "on-exception"
		}`,
	}.Test()
}
