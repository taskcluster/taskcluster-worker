// +build linux,docker

package dockerengine

import (
	"os"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

// Image and tag used in test cases below
const (
	dockerImageName = "alpine:3.6"
)

var provider = &enginetest.EngineProvider{
	Engine: "docker",
	Config: `{
		"privileged": "allow"
	}`,
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

func TestLogging(t *testing.T) {
	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		TargetPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && true"],
			"image": "` + dockerImageName + `"
		}`,
		FailingPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && false"],
			"image": "` + dockerImageName + `"
		}`,
		SilentPayload: `{
			"command": ["sh", "-c", "echo 'no hello' && true"],
			"image": "` + dockerImageName + `"
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
			"image": "` + dockerImageName + `"
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
			"image": "` + dockerImageName + `"
		}`,
	}

	c.Test()
}

func TestArtifacts(t *testing.T) {
	c := enginetest.ArtifactTestCase{
		EngineProvider:     provider,
		Text:               "[hello-world]",
		TextFilePath:       "/folder/hello.txt",
		FileNotFoundPath:   "/no-such-file.txt",
		FolderNotFoundPath: "/no-such-folder",
		NestedFolderFiles: []string{
			"hello.txt",
			"sub-folder/hello2.txt",
		},
		NestedFolderPath: "/folder",
		Payload: `{
			"command": ["sh", "-ec", "mkdir -p /folder/sub-folder; echo '[hello-world]' > /folder/hello.txt; echo '[hello-world]' > /folder/sub-folder/hello2.txt"],
			"image": "` + dockerImageName + `"
		}`,
	}

	c.Test()
}

func TestProxies(t *testing.T) {
	c := enginetest.ProxyTestCase{
		EngineProvider: provider,
		ProxyName:      "my-proxy",
		PingProxyPayload: `{
			"command": ["sh", "-ec", "` +
			`apk add --no-cache curl > /dev/null; ` +
			`echo 'Pinging'; ` +
			`STATUS=$(curl -s -o /tmp/output -w '%{http_code}' http://taskcluster/my-proxy/v1/ping); ` +
			`cat /tmp/output; ` +
			`test $STATUS -eq 200;` +
			`"],
			"image": "` + dockerImageName + `"
		}`,
	}

	c.Test()
}

func TestPrivileged(t *testing.T) {
	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Scopes:         []string{"worker:privileged:*"},
		Target:         "ffffffff", // CapInh won't have many f's if it's not privileged
		TargetPayload: `{
			"command": ["sh", "-c", "cat /proc/1/status | grep CapInh"],
			"privileged": true,
			"image": "` + dockerImageName + `"
		}`,
		FailingPayload: `{
			"command": ["sh", "-c", "cat /proc/1/status | grep CapInh && false"],
			"privileged": true,
			"image": "` + dockerImageName + `"
		}`,
		SilentPayload: `{
			"command": ["sh", "-c", "cat /proc/1/status | grep CapInh"],
			"privileged": false,
			"image": "` + dockerImageName + `"
		}`,
	}

	c.Test()
}

func TestVolumes(t *testing.T) {
	c := enginetest.VolumeTestCase{
		EngineProvider: provider,
		Mountpoint:     "/mnt/my-volume/",
		WriteVolumePayload: `{
			"image": "` + dockerImageName + `",
			"command": ["sh", "-c", "echo 'hello-cache-volume' > /mnt/my-volume/cache-file.txt"]
		}`,
		CheckVolumePayload: `{
			"image": "` + dockerImageName + `",
			"command": ["sh", "-c", "cat /mnt/my-volume/cache-file.txt | grep 'hello-cache-volume'"]
		}`,
	}

	c.Test()
}
