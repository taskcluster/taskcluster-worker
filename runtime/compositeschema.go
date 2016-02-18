package runtime

import (
	"encoding/json"
	"errors"

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
					if entry.property == e.property && entry.schema != e.schema {
						// TODO: We probably should make an error with a custom message
						hasConflict = true
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
			return nil, errors.New("Property is missing")
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
