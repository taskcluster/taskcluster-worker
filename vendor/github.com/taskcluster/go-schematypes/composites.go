package schematypes

import "reflect"

// An AnyOf instance represents the anyOf JSON schema construction.
type AnyOf []Schema

// A OneOf instance represents the oneOf JSON schema construction.
type OneOf []Schema

// An AllOf instance represents the allOf JSON schema construction.
type AllOf []Schema

// Schema returns a JSON representation of the schema.
func (s AnyOf) Schema() map[string]interface{} {
	schemas := make([]interface{}, len(s))
	for i, schema := range s {
		schemas[i] = schema.Schema()
	}
	return map[string]interface{}{"anyOf": schemas}
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (s AnyOf) Validate(data interface{}) error {
	for _, schema := range s {
		if schema.Validate(data) == nil {
			return nil
		}
	}
	return singleIssue("", "None of the anyOf options at {path} was satisfied")
}

// Map takes data, validates and maps it into the target reference.
func (s AnyOf) Map(data, target interface{}) error {
	return mapToEmptyInterface(s, data, target)
}

// Schema returns a JSON representation of the schema.
func (s OneOf) Schema() map[string]interface{} {
	schemas := make([]interface{}, len(s))
	for i, schema := range s {
		schemas[i] = schema.Schema()
	}
	return map[string]interface{}{"oneOf": schemas}
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (s OneOf) Validate(data interface{}) error {
	satisfied := 0
	for _, schema := range s {
		if schema.Validate(data) == nil {
			satisfied++
		}
	}
	if satisfied == 0 {
		return singleIssue("", "None of the oneOf options at {path} was satisfied")
	}
	if satisfied > 1 {
		return singleIssue("", "More than one of the oneOf options at {path} was satisfied")
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (s OneOf) Map(data, target interface{}) error {
	return mapToEmptyInterface(s, data, target)
}

// Schema returns a JSON representation of the schema.
func (s AllOf) Schema() map[string]interface{} {
	schemas := make([]interface{}, len(s))
	for i, schema := range s {
		schemas[i] = schema.Schema()
	}
	return map[string]interface{}{"allOf": schemas}
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (s AllOf) Validate(data interface{}) error {
	for _, schema := range s {
		if err := schema.Validate(data); err != nil {
			return err
		}
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (s AllOf) Map(data, target interface{}) error {
	return mapToEmptyInterface(s, data, target)
}

func mapToEmptyInterface(s Schema, data, target interface{}) error {
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
