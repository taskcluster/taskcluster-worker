package runtime

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/taskcluster/taskcluster-base-go/jsontest"
	"github.com/xeipuuv/gojsonschema"
)

// ErrConflictingSchemas is returned if two schema entries are conflicting.
var ErrConflictingSchemas = errors.New("Two schema entries are conflicting!")

// CompositeSchema hides one or more composed JSON schemas.
type CompositeSchema interface {
	Parse(data map[string]json.RawMessage) (interface{}, error)
	visit(visitor func(*schemaEntry))
}

type emptySchema struct{}

type schemaEntry struct {
	property   string
	schema     string
	required   bool
	makeTarget func() interface{}
	validator  *gojsonschema.Schema
}

type composedSchema []CompositeSchema

// NewEmptyCompositeSchema returns a CompositeSchema schema that is empty.
// The resulting value from Parse is nil, and the schema does no validation.
func NewEmptyCompositeSchema() CompositeSchema {
	return &emptySchema{}
}

func (*emptySchema) Parse(map[string]json.RawMessage) (interface{}, error) {
	return nil, nil
}

func (*emptySchema) visit(func(*schemaEntry)) {}

// NewCompositeSchema creates a CompositeSchema from the description of a single
// property and a function to produce unmarshalling targets with.
//
// Schema will only validate the 'property' against the JSON schema passed as
// 'schema'. If the 'required' is true the property must be present.
//
// When parsing the makeTarget will be used as factory to create objects
// which the payload will unmarshalled to.
func NewCompositeSchema(
	property string,
	schema string,
	required bool,
	makeTarget func() interface{},
) (CompositeSchema, error) {
	validator, err := gojsonschema.NewSchema(gojsonschema.NewStringLoader(schema))
	if err != nil {
		return nil, err
	}
	return &schemaEntry{
		property:   property,
		schema:     schema,
		required:   required,
		makeTarget: makeTarget,
		validator:  validator,
	}, nil
}

// MergeCompositeSchemas will merge two or more CompositeSchema
//
// When CompositeSchema.Parse is called it will return an array of the results
// from the schemas that were merged. Hence, the order in which the schemas is
// given is important and will be preserved.
//
// This function may return ErrConflictingSchemas, if two of the schemas merged
// have conflicting definitions.
func MergeCompositeSchemas(schemas ...CompositeSchema) (CompositeSchema, error) {
	hasConflict := false
	for i, schema := range schemas {
		schema.visit(func(entry *schemaEntry) {
			for _, s := range schemas[i:] {
				s.visit(func(e *schemaEntry) {
					if entry.property == e.property {
						if schemasMatch, _, _, _ := jsontest.JsonEqual([]byte(entry.schema), []byte(e.schema)); !schemasMatch {
							// TODO: We probably should make an error with a custom message
							hasConflict = true
						}
					}
				})
			}
		})
	}
	if hasConflict {
		return nil, ErrConflictingSchemas
	}
	return composedSchema(schemas), nil
}

// Parse will validate and parse data.
//
// This method will return an object returned from makeTarget (or )
func (s *schemaEntry) Parse(data map[string]json.RawMessage) (interface{}, error) {
	// TODO: Validate property against schema
	value := data[s.property]
	if value == nil {
		if s.required {
			return nil, errors.New("Property \"" + s.property + "\" is missing")
		}
		return nil, nil
	}

	// Validate value against json schema
	result, err := s.validator.Validate(gojsonschema.NewStringLoader(string(value)))
	if err != nil {
		return nil, err
	}
	// Check for validation errors
	if !result.Valid() {
		message := "JSON schema validation failed:\n"
		for _, err := range result.Errors() {
			// Err implements the ResultError interface
			message += err.Description() + "\n"
		}
		return nil, errors.New(message)
	}

	// Unmarshal value to target
	target := s.makeTarget()
	err = json.Unmarshal(value, target)
	if err != nil {
		return nil, err
	}
	return target, nil
}

func (s *schemaEntry) visit(visitor func(entry *schemaEntry)) {
	visitor(s)
}

func (s composedSchema) Parse(data map[string]json.RawMessage) (interface{}, error) {
	results := []interface{}{}
	for _, entry := range s {
		result, err := entry.Parse(data)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (s composedSchema) visit(visitor func(*schemaEntry)) {
	for _, entry := range s {
		entry.visit(visitor)
	}
}

// Schema returns the composed JSON schema from a CompositeSchema object
func Schema(schema CompositeSchema, id, title, description string) string {
	properties := make(map[string]*json.RawMessage)
	required := []string{}

	schema.visit(func(entry *schemaEntry) {
		s := json.RawMessage(entry.schema)
		properties[entry.property] = &s

		// We panic if there is two entries for the same property with different
		// schemas... This shouldn't be possible as MergeCompositeSchemas checks it.
		if properties[entry.property] != nil {
			schemasMatch, _, _, _ := jsontest.JsonEqual(s, []byte(*properties[entry.property]))
			if !schemasMatch {
				panic(fmt.Sprint(
					"It shouldn't be possible to construct a CompositeSchema with two ",
					"different schemas for the same property. This happend for ",
					"property: ", entry.property, " where: ", entry.schema, " is ",
					"different from: ", string(*properties[entry.property]),
				))
			}
		}

		// If property is required and not already in the list we append it
		if entry.required {
			contains := false
			for _, property := range required {
				contains = contains || property == entry.property
			}
			if !contains {
				required = append(required, entry.property)
			}
		}
	})

	// Marshal to JSON, as this is generated for export we will intent it, just
	// in case some human decides to read it.
	json, err := json.MarshalIndent(struct {
		ID                   string                      `json:"id"`
		Schema               string                      `json:"$schema"`
		Title                string                      `json:"title"`
		Description          string                      `json:"description"`
		Type                 string                      `json:"type"`
		Properties           map[string]*json.RawMessage `json:"properties,omitempty"`
		Required             []string                    `json:"required,omitempty"`
		AdditionalProperties bool                        `json:"additionalProperties"`
	}{
		ID:                   id,
		Schema:               "http://json-schema.org/draft-04/schema#",
		Title:                title,
		Description:          description,
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: false,
	}, "", "  ")
	// All properties should be JSON, this should always render to valid JSON.
	// Errors can't happen here, they should happen in NewCompositeSchema
	if err != nil {
		panic(fmt.Sprint("Rendering composite schema to JSON failed: ", err))
	}

	return string(json)
}
