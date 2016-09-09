package schematypes

import (
	"math"
	"reflect"
)

// Map specifies schema for a map from string to values.
type Map struct {
	MetaData
	Values            Schema
	MinimumProperties int64
	MaximumProperties int64
}

// Schema returns a JSON representation of the schema.
func (m Map) Schema() map[string]interface{} {
	s := m.schema()
	s["type"] = "object"
	s["additionalProperties"] = m.Values.Schema()
	if m.MinimumProperties != 0 {
		s["minProperties"] = m.MinimumProperties
	}
	if m.MaximumProperties != math.MaxInt64 && m.MaximumProperties != 0 {
		s["maxProperties"] = m.MaximumProperties
	}
	return s
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (m Map) Validate(data interface{}) error {
	value, ok := data.(map[string]interface{})
	if !ok {
		return singleIssue("", "Expected object type at {path}")
	}

	e := &ValidationError{}

	for key, value := range value {
		e.addIssuesWithPrefix(m.Values.Validate(value), formatKeyPath(key))
	}
	if m.MinimumProperties > int64(len(value)) {
		e.addIssue("",
			"Expected a minimum of %d properties at {path}, but only found %d properties",
			m.MinimumProperties, len(value),
		)
	}
	if m.MaximumProperties != 0 && m.MaximumProperties < int64(len(value)) {
		e.addIssue("",
			"Expected a maximum of %d properties at {path}, but found %d properties",
			m.MaximumProperties, len(value),
		)
	}

	if len(e.issues) > 0 {
		return e
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (m Map) Map(data, target interface{}) error {
	if err := m.Validate(data); err != nil {
		return err
	}

	// Ensure that we have a pointer as input
	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	// Ensure the type is a map from string to something
	if val.Kind() != reflect.Map || val.Type().Key().Kind() != reflect.String {
		return ErrTypeMismatch
	}

	// Create a new map
	val.Set(reflect.MakeMap(val.Type()))

	// Set (key, value) pairs in the result
	valueType := val.Type().Elem()
	for key, value := range data.(map[string]interface{}) {

		var targetValue reflect.Value
		var resultValue reflect.Value
		if valueType.Kind() == reflect.Ptr {
			targetValue = reflect.New(valueType)
			resultValue = targetValue
		} else if valueType == typeOfEmptyInterface {
			val.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(value))
			continue
		} else {
			targetValue = reflect.New(valueType)
			resultValue = targetValue.Elem()
		}

		if err := m.Values.Map(value, targetValue.Interface()); err != nil {
			if err != ErrTypeMismatch {
				panic("Internal error, this should have been caught in Validate()")
			}
			return err
		}
		val.SetMapIndex(reflect.ValueOf(key), resultValue)
	}

	return nil
}
