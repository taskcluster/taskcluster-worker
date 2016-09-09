package schematypes

import "fmt"

// stringContains returns true if list contains element
func stringContains(list []string, element string) bool {
	for _, s := range list {
		if s == element {
			return true
		}
	}
	return false
}

// MustMap will map data into target using schema and panic, if it returns
// ErrTypeMismatch
func MustMap(schema Schema, data, target interface{}) error {
	err := schema.Map(data, target)
	if err == ErrTypeMismatch {
		panic(fmt.Sprintf(
			"ErrTypeMismatch, target type: %#v doesn't match schema: %#v",
			target, schema,
		))
	}
	return err
}
