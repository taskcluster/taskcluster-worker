package runtime

import (
	"encoding/json"
	"fmt"
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

func TestCreateMergedCompositeSchemas(t *testing.T) {
	t.Parallel()

	c1, err := NewCompositeSchema("prop1", `{
		"type": "object",
        "properties": {
			"count": {"type": "integer"}
        },
		"additionalProperties": false
	}`, true, func() interface{} { return nil })

	if err != nil {
		t.Fatal(err)
	}

	c2, err := NewCompositeSchema("prop2", `{
		"type": "object",
        "properties": {
			"count": {"type": "integer"}
        },
		"additionalProperties": false
	}`, true, func() interface{} { return nil })

	if err != nil {
		t.Fatal(err)
	}

	if _, err = MergeCompositeSchemas(c1, c2); err != nil {
		t.Fatalf("Error creating a merged composite scheme. %v", err)
	}
}

func TestMergeComposedSchemas(t *testing.T) {
	t.Parallel()

	schema := `
		{
			"type": "object",
			"properties": {
				"count": {"type": "integer"}
			},
			"additionalProperties": false
		}
	`

	composites := make([]CompositeSchema, 4)
	for i := range composites {
		// Create unique property names to avoid collision resulting in error
		propertyName := fmt.Sprintf("prop%d", i)
		composite, err := NewCompositeSchema(propertyName, schema, true, func() interface{} { return nil })
		if err != nil {
			t.Fatal(err)
		}
		composites[i] = composite
	}

	composed1, err := MergeCompositeSchemas(composites[0:2]...)
	if err != nil {
		t.Fatal(err)
	}

	composed2, err := MergeCompositeSchemas(composites[2:]...)
	if err != nil {
		t.Fatal(err)
	}

	merged, err := MergeCompositeSchemas(composed1, composed2)
	if err != nil {
		t.Fatalf("Error creating a merged composite scheme. %v", err)
	}

	m := merged.(composedSchema)
	if len(m) != 2 {
		t.Fatalf("Merged schema should have 2 entries but had %d", len(m))
	}
}

func TestConflictingMergedCompositeSchemas(t *testing.T) {
	c1, err := NewCompositeSchema("prop", `{
		"type": "object",
        "properties": {
			"count": {"type": "integer"}
        },
		"additionalProperties": false
	}`, true, func() interface{} { return nil })

	if err != nil {
		t.Fatal(err)
	}

	// Create a schema that has the same property name, but a different schema.
	c2, err := NewCompositeSchema("prop", `{
		"type": "object",
        "properties": {
			"id": {"type": "integer"}
        },
		"additionalProperties": false
	}`, true, func() interface{} { return nil })

	if err != nil {
		t.Fatal(err)
	}

	if _, err = MergeCompositeSchemas(c1, c2); err == nil {
		t.Fatalf("Merged composite schema should not have been created.")
	}
}

func TestRequiredSchemaPropertyMissing(t *testing.T) {
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
	jsonMessage := `
		{
			"prop1": {
			  "count": 42
			},
			"other": 55
		}
	`
	if err = json.Unmarshal([]byte(jsonMessage), &data); err != nil {
		t.Fatal("test definition error", err)
	}

	if _, err = c.Parse(data); err == nil {
		t.Fatalf("Error should have been returned for missing property. Received: '%s'", err)
	}
}

func TestUnrequiredSchemaPropertyMissing(t *testing.T) {
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
  }`, false, func() interface{} { return &Target{} })
	if err != nil {
		t.Fatal(err)
	}

	// Parse something (all this happens in one place only)
	data := map[string]json.RawMessage{}
	jsonMessage := `
		{
			"prop1": {
			  "count": 42
			},
			"other": 55
		}
	`
	if err = json.Unmarshal([]byte(jsonMessage), &data); err != nil {
		t.Fatal("test definition error", err)
	}

	if result, err := c.Parse(data); err != nil || result != nil {
		t.Fatal("Error and result should be nil when unrequired property not found")
	}
}

func TestSchemaUnmarshalToInvalidType(t *testing.T) {
	t.Parallel()

	type Target struct {
		//  Trying to unmarshal an int to a string datatype should
		//  cause a validation error
		Count string `json:"count"`
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

	if _, err = c.Parse(data); err == nil {
		t.Fatal("Error should have been reported when unmarshalling an int to a string.")
	}
}
