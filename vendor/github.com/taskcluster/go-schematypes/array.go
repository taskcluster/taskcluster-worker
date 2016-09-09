package schematypes

import (
	"fmt"
	"reflect"
)

// An Array struct represents the JSON schema for an array.
type Array struct {
	MetaData
	Items  Schema
	Unique bool
}

// Schema returns a JSON representation of the schema.
func (a Array) Schema() map[string]interface{} {
	m := a.schema()
	m["type"] = "array"
	m["items"] = a.Items.Schema()
	if a.Unique {
		m["uniqueItems"] = a.Unique
	}
	return m
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (a Array) Validate(data interface{}) error {
	value := reflect.ValueOf(data)

	if value.Kind() != reflect.Array && value.Kind() != reflect.Slice {
		return singleIssue("", "Expected array or slice at {path}")
	}

	e := &ValidationError{}

	// Validate all elements
	N := value.Len()
	for i := 0; i < N; i++ {
		vi := value.Index(i).Interface()
		e.addIssuesWithPrefix(a.Items.Validate(vi), "[%d]", i)

		// Test for uniqueness if required
		if a.Unique {
			// If testing we start from i + 1, so we don't test with i. Besides we
			// can safely assume everything before i has been test against i.
			for j := i + 1; j < N; j++ {
				vj := value.Index(j).Interface()
				if reflect.DeepEqual(vi, vj) {
					e.addIssue(fmt.Sprintf("[%d]", i),
						"Array doesn't have unique items, index %d and %d are equal", i, j)
					break
				}
			}
		}
	}

	if len(e.issues) > 0 {
		return e
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (a Array) Map(data, target interface{}) error {
	if err := a.Validate(data); err != nil {
		return err
	}

	// Ensure that we have a pointer as input
	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	// Ensure that we have an array type
	if val.Kind() != reflect.Slice {
		return ErrTypeMismatch
	}
	elem := val.Type().Elem()

	// Set out to length zero
	val.SetLen(0)

	// For each element, use Map from items-schema to construct sub-element
	value := reflect.ValueOf(data)
	N := value.Len()
	for i := 0; i < N; i++ {
		val.Set(reflect.Append(val, reflect.Zero(elem)))
		v := value.Index(i).Interface()
		vt := val.Index(i).Addr().Interface()
		if err := a.Items.Map(v, vt); err != nil {
			if err != ErrTypeMismatch {
				panic("Internal error, this should have been caught in Validate()")
			}
			return err
		}
	}

	return nil
}
