package configenv

import (
	"os"
	"testing"

	"github.com/taskcluster/taskcluster-worker/config/configtest"
)

func TestEnvTransform(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", "hello-world")
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$env": "CFG_TEST_ENV_VAR"},
		},
		Result: map[string]interface{}{
			"key": "hello-world",
		},
	}.Test()
}
