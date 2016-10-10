package schematypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// A Schema is implemented by any object that can represent a JSON schema.
type Schema interface {
	Schema() map[string]interface{}
	Validate(data interface{}) error
	Map(data, target interface{}) error
}

// NewSchema creates a Schema that wraps a jsonschema.
// The jsonschema can be specified as a JSON string or a hierarchy of
//   * map[string]interface{}
//   * []interface{}
//   * string
//   * float64
//   * bool
//   * nil
func NewSchema(jsonschema interface{}) (Schema, error) {
	var loader gojsonschema.JSONLoader
	if s, ok := jsonschema.(string); ok {
		var target interface{}
		if err := json.Unmarshal([]byte(s), &target); err != nil {
			return nil, fmt.Errorf("Failed to parse JSON string, error: %s", err)
		}
		jsonschema = target
	}
	obj, ok := jsonschema.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Expected map[string]interface{} got: %T", jsonschema)
	}
	loader = gojsonschema.NewGoLoader(jsonschema)

	s, err := gojsonschema.NewSchema(loader)
	if err != nil {
		return nil, err
	}

	return schema{schema: s, raw: obj}, nil
}

type schema struct {
	schema *gojsonschema.Schema
	raw    map[string]interface{}
}

func (s schema) Schema() map[string]interface{} {
	return s.raw
}

func (s schema) Validate(data interface{}) error {
	result, _ := s.schema.Validate(gojsonschema.NewGoLoader(data))
	if result.Valid() {
		return nil
	}
	msgs := []string{}
	for _, e := range result.Errors() {
		msgs = append(msgs, e.Description())
	}

	return singleIssue("", "Faild to validate sub-schema at {path}, errors: %s",
		strings.Join(msgs, ", "),
	)
}

func (s schema) Map(data, target interface{}) error {
	if err := s.Validate(data); err != nil {
		return err
	}

	// Ensure that we have a pointer as input
	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	if val.Type() != typeOfEmptyInterface {
		return ErrTypeMismatch
	}

	val.Set(reflect.ValueOf(data))

	return nil
}
