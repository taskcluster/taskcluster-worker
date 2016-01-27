package runtime

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSingleRequiredSchema(t *testing.T) {
	t.Parallel()
	// Create composite schema, this is part of the plugin/engine implementations
	type Target struct {
		Count int `json:"count"`
	}
	c, err := NewCompositeSchema("prop", `{
    "type": "object",
    "properties": {
      "count": {"type": "integer"}
    },
    "additionalProperties": false
  }`, true, func() interface{} { return &Target{} })
	if err != nil {
		t.Fatal(err)
	}

	// Parse something (all this happens in one place only)
	data := map[string]json.RawMessage{}
	err = json.Unmarshal([]byte(`{
    "prop": {
      "count": 42
    },
    "other": 55
  }`), &data)
	if err != nil {
		t.Fatal("test definition error", err)
	}
	result, err := c.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// This is part that makes into the plugin/engine when they handle payload
	// that was parsed...
	target, found := result.(*Target)
	if !found {
		t.Error("We specified required as true, so this shouldn't be possible")
	}
	if target.Count != 42 {
		t.Error("Expected 42")
	}
}

func TestEmptyCompositeSchema(t *testing.T) {
	t.Parallel()

	ec := NewEmptyCompositeSchema()
	if reflect.TypeOf(ec).String() != "*runtime.emptySchema" {
		t.Fatal("Empty schema not created")
	}

	type Target struct {
		Count int `json:"count"`
	}

	// Parse something (all this happens in one place only)
	data := map[string]json.RawMessage{}
	result, err := ec.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if result != nil {
		t.Fatalf("Result should have been nil, but got %s", result)
	}
}

func TestInvalidSchemaReference(t *testing.T) {
	t.Parallel()

	invalidSchema := `
		{
			"type": "object",
			"properties": {
		}
	`
	schema, err := NewCompositeSchema("prop", invalidSchema, true, func() interface{} { return nil })
	if schema != nil {
		t.Fatal("Schema should not have been created with an invalid schema reference")
	}

	if err == nil {
		t.Fatal("Error not returned indicating a composite schema could not be created")
	}

}
