package schematypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Properties defines the properties for a object schema.
type Properties map[string]Schema

// Object specifies schema for an object.
type Object struct {
	MetaData
	Properties           Properties
	AdditionalProperties bool
	Required             []string
}

// Schema returns a JSON representation of the schema.
func (o Object) Schema() map[string]interface{} {
	m := o.schema()
	m["type"] = "object"
	if len(o.Properties) > 0 {
		props := make(map[string]map[string]interface{})
		for prop, schema := range o.Properties {
			props[prop] = schema.Schema()
		}
		m["properties"] = props
	}
	if !o.AdditionalProperties {
		m["additionalProperties"] = o.AdditionalProperties
	}
	if len(o.Required) > 0 {
		m["required"] = o.Required
	}
	return m
}

var identifierPattern = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")

func formatKeyPath(key string) string {
	if identifierPattern.MatchString(key) {
		return "." + key
	}
	j, _ := json.Marshal(key)
	return "[" + string(j) + "]"
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (o Object) Validate(data interface{}) error {
	value, ok := data.(map[string]interface{})
	if !ok {
		return singleIssue("", "Expected object type at {path}")
	}

	e := ValidationError{}

	// Test properties
	for p, s := range o.Properties {
		v, ok := value[p]
		if !ok {
			continue
		}
		if err := s.Validate(v); err != nil {
			e.addIssuesWithPrefix(err, formatKeyPath(p))
		}
	}

	// Test for additional properties
	if !o.AdditionalProperties {
		for key := range value {
			if _, ok := o.Properties[key]; !ok {
				e.addIssue(formatKeyPath(key), "Additional property '%s' not allowed at {path}", key)
			}
		}
	}

	// Test required properties
	for _, key := range o.Required {
		if _, ok := value[key]; !ok {
			e.addIssue(formatKeyPath(key), "Required property '%s' is missing at {path}", key)
		}
	}

	if len(e.issues) > 0 {
		return &e
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (o Object) Map(data, target interface{}) error {
	if err := o.Validate(data); err != nil {
		return err
	}

	// Ensure that we have a pointer as input
	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	// Use mapStruct if we have a struct type
	if val.Kind() == reflect.Struct {
		return o.mapStruct(data.(map[string]interface{}), val)
	}
	if val.Kind() == reflect.Map && val.Type().Key().Kind() == reflect.String {
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

			schema := o.Properties[key]
			if schema == nil {
				continue // can't map if there is no schema
			}
			if err := schema.Map(value, targetValue.Interface()); err != nil {
				if err != ErrTypeMismatch {
					panic("Internal error, this should have been caught in Validate()")
				}
				return err
			}
			val.SetMapIndex(reflect.ValueOf(key), resultValue)
		}
		return nil
	}

	return ErrTypeMismatch
}

func jsonTag(field reflect.StructField) string {
	j := field.Tag.Get("json")
	if strings.HasSuffix(j, ",omitempty") {
		return j[:len(j)-10]
	}
	return j
}

func hasStructTag(t reflect.Type, tag string) bool {
	N := t.NumField()
	for i := 0; i < N; i++ {
		f := t.Field(i)
		j := jsonTag(f)
		if j == tag {
			return true
		} else if j == "" && f.Name == tag {
			return true
		}
	}
	return false
}

var typeOfEmptyInterface = reflect.TypeOf((*interface{})(nil)).Elem()

func (o Object) mapStruct(data map[string]interface{}, target reflect.Value) error {
	t := target.Type()

	// We have a type mismatch if there isn't fields for the values declared
	for key := range o.Properties {
		if !hasStructTag(t, key) {
			return ErrTypeMismatch
		}
	}

	for _, key := range o.Required {
		if !hasStructTag(t, key) {
			return ErrTypeMismatch
		}
	}

	N := t.NumField()
	for i := 0; i < N; i++ {
		// Find field and json tag
		f := t.Field(i)
		tag := jsonTag(f)

		// Find value, if there is one
		value, ok := data[tag]
		if !ok {
			continue // We've already validated, so no need to check required
		}

		// Find schema for property, ignore if there is none
		s := o.Properties[tag]
		if s == nil {
			continue
		}

		var targetValue reflect.Value
		if f.Type.Kind() == reflect.Ptr {
			targetValue = reflect.New(f.Type.Elem())
			target.Field(i).Set(targetValue)
		} else if f.Type == typeOfEmptyInterface {
			target.Field(i).Set(reflect.ValueOf(value))
			continue
		} else {
			targetValue = target.Field(i).Addr()
		}

		// Map value to field
		err := s.Map(value, targetValue.Interface())
		if err != nil {
			return err
		}
	}

	return nil
}

// Filter will create a new map with any additional properties that aren't
// allowed by the schema removed. This doesn't modify the data parameter, but
// returns a new map.
//
// Note: Naturally this have no effect if AdditionalProperties is true.
func (o Object) Filter(data map[string]interface{}) map[string]interface{} {
	value := make(map[string]interface{})
	for k, v := range data {
		if _, ok := o.Properties[k]; ok || o.AdditionalProperties {
			value[k] = v
		}
	}
	return value
}

// Merge multiple object schemas, this will create an object schema with all the
// properties from the schemas given, and all the required properties as the
// given object schemas have.
//
// This will fail if any schema has AdditionalProperties: true, or if any two
// schemas specifies the same key with different schemas.
//
// When using this to merge multiple schemas into one schema, the Object.Filter
// method may be useful to strip forbidden properties such that the subset of
// values matching a specific schema used can be extract for use with
// Object.Validate or Object.Map.
func Merge(a ...Object) (Object, error) {
	props := make(map[string]Schema)
	required := []string{}

	for _, obj := range a {
		// Return an error if AdditionalProperties is set
		if obj.AdditionalProperties {
			return Object{}, fmt.Errorf("AdditionalProperties is true for %#v", obj)
		}

		// Merge the properties from all objects, returning an error if there is
		// two objects with different schemas for the same property
		for k, schema := range obj.Properties {
			existing, ok := props[k]
			if !ok {
				props[k] = schema
			} else if !reflect.DeepEqual(schema, existing) {
				return Object{}, fmt.Errorf(
					"The key '%s' is defined with different schemas %#v and %#v",
					k, schema, existing,
				)
			}
		}

		// Merge the lists of requried properties
		for _, k := range obj.Required {
			if !stringContains(required, k) {
				required = append(required, k)
			}
		}
	}

	return Object{
		Properties:           props,
		Required:             required,
		AdditionalProperties: false,
	}, nil
}
