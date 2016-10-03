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
	}.Test(t)
}

func TestEnvTransformArray(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", "hello-world")
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": []interface{}{
				"test",
				map[string]interface{}{"$env": "CFG_TEST_ENV_VAR"},
			},
		},
		Result: map[string]interface{}{
			"key": []interface{}{
				"test",
				"hello-world",
			},
		},
	}.Test(t)
}

func TestEnvTransformString(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", "hello-world")
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$env": "CFG_TEST_ENV_VAR", "type": "string"},
		},
		Result: map[string]interface{}{
			"key": "hello-world",
		},
	}.Test(t)
}

func TestEnvTransformNumber(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", "52")
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$env": "CFG_TEST_ENV_VAR", "type": "number"},
		},
		Result: map[string]interface{}{
			"key": float64(52),
		},
	}.Test(t)
}

func TestEnvTransformJSON(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", `{"hello": [1, 2, "test"]}`)
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$env": "CFG_TEST_ENV_VAR", "type": "json"},
		},
		Result: map[string]interface{}{
			"key": map[string]interface{}{
				"hello": []interface{}{
					float64(1),
					float64(2),
					"test",
				},
			},
		},
	}.Test(t)
}

func TestEnvTransformBooleanTrue(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", "true")
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$env": "CFG_TEST_ENV_VAR", "type": "bool"},
		},
		Result: map[string]interface{}{
			"key": true,
		},
	}.Test(t)
}

func TestEnvTransformBooleanFalse(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", "false")
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$env": "CFG_TEST_ENV_VAR", "type": "bool"},
		},
		Result: map[string]interface{}{
			"key": false,
		},
	}.Test(t)
}

func TestEnvTransformList(t *testing.T) {
	os.Setenv("CFG_TEST_ENV_VAR", "hello world")
	configtest.Case{
		Transform: "env",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$env": "CFG_TEST_ENV_VAR", "type": "list"},
		},
		Result: map[string]interface{}{
			"key": []interface{}{"hello", "world"},
		},
	}.Test(t)
}
