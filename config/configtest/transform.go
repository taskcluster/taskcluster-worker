// Package configtest provides structs and logic for declarative configuration
// tests.
package configtest

import (
	"fmt"
	"reflect"

	"github.com/taskcluster/taskcluster-worker/config"
)

// Case allows declaration of a transformation to run on input and validate
// against declared tesult.
type Case struct {
	Transform string
	Input     map[string]interface{}
	Result    map[string]interface{}
}

// Test will execute the test case panicing if Input doesn't become Result
func (c Case) Test() {
	transform := config.Providers()[c.Transform]
	if transform == nil {
		panic(fmt.Sprintf("Unknown transform: %s", c.Transform))
	}

	err := transform.Transform(c.Input)
	if err != nil {
		panic(fmt.Sprintf("Transform(Input) failed, error: %s", err))
	}

	if !reflect.DeepEqual(c.Input, c.Result) {
		panic(fmt.Sprintf("c.Result != c.Input, after transform, result: %#v ", c.Input))
	}
}
