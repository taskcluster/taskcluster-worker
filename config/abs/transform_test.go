package configabs

import (
	"path/filepath"
	"testing"

	"github.com/taskcluster/taskcluster-worker/config/configtest"
)

func TestAbsTransform(t *testing.T) {
	result, _ := filepath.Abs("transform.go")
	configtest.Case{
		Transform: "abs",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$abs": "../abs/transform.go"},
		},
		Result: map[string]interface{}{
			"key": result,
		},
	}.Test(t)
}
