package runtime

import (
	"encoding/json"
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
