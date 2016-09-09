package schematypes

import (
	"math"
	"net/url"
	"reflect"
	"regexp"
	"time"
)

const (
	typeInteger = "integer"
	typeString  = "string"
	typeNumber  = "number"
	typeBoolean = "boolean"
)

// The Integer struct represents a JSON schema for an integer.
type Integer struct {
	MetaData
	Minimum int64
	Maximum int64
}

// Schema returns a JSON representation of the schema.
func (i Integer) Schema() map[string]interface{} {
	m := i.schema()
	m["type"] = typeInteger
	if i.Minimum != math.MinInt64 {
		m["minimum"] = i.Minimum
	}
	if i.Maximum != math.MaxInt64 {
		m["maximum"] = i.Maximum
	}
	return m
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (i Integer) Validate(data interface{}) error {
	var value int64
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if float64(int64(v.Float())) != v.Float() {
			return singleIssue("", "Expected an integer at {path}")
		}
		value = int64(v.Float())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value = v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value = int64(v.Uint())
	default:
		return singleIssue("", "Expected an integer at {path}")
	}

	if value < i.Minimum {
		return singleIssue("", "Integer %d at {path} is less than minimum %d",
			value, i.Minimum,
		)
	}
	if value > i.Maximum {
		return singleIssue("", "Integer %d at {path} is larger than maximum %d",
			value, i.Maximum,
		)
	}

	return nil
}

// Map takes data, validates and maps it into the target reference.
func (i Integer) Map(data interface{}, target interface{}) error {
	if err := i.Validate(data); err != nil {
		return err
	}

	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	var value int64
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		value = int64(v.Float())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value = v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value = int64(v.Uint())
	default:
		panic("internal error -- validate should have caught this")
	}

	switch val.Kind() {
	case reflect.Int8:
		if i.Minimum < math.MinInt8 && i.Maximum > math.MaxInt8 {
			return ErrTypeMismatch
		}
		fallthrough
	case reflect.Int16:
		if i.Minimum < math.MinInt16 && i.Maximum > math.MaxInt16 {
			return ErrTypeMismatch
		}
		fallthrough
	case reflect.Int32, reflect.Int:
		if i.Minimum < math.MinInt32 && i.Maximum > math.MaxInt32 {
			return ErrTypeMismatch
		}
		fallthrough
	case reflect.Int64:
		val.SetInt(value)
		return nil
	case reflect.Uint8:
		if i.Minimum < 0 && i.Maximum > math.MaxUint8 {
			return ErrTypeMismatch
		}
		fallthrough
	case reflect.Uint16:
		if i.Minimum < 0 && i.Maximum > math.MaxUint16 {
			return ErrTypeMismatch
		}
		fallthrough
	case reflect.Uint32, reflect.Uint:
		if i.Minimum < 0 && i.Maximum > math.MaxUint32 {
			return ErrTypeMismatch
		}
		fallthrough
	case reflect.Uint64:
		if i.Minimum < 0 {
			return ErrTypeMismatch
		}
		val.SetUint(uint64(value))
		return nil
	default:
		return ErrTypeMismatch
	}
}

// Number schema type.
type Number struct {
	MetaData
	Minimum float64
	Maximum float64
}

// Schema returns a JSON representation of the schema.
func (n Number) Schema() map[string]interface{} {
	m := n.schema()
	m["type"] = typeNumber
	if n.Minimum != -math.MaxFloat64 {
		m["minimum"] = n.Minimum
	}
	if n.Maximum != math.MaxFloat64 {
		m["maximum"] = n.Maximum
	}
	return m
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (n Number) Validate(data interface{}) error {
	value, ok := data.(float64)
	if !ok {
		return singleIssue("", "Expected a number at {path}")
	}
	if value < n.Minimum {
		return singleIssue("", "Number %d at {path} is less than minimum %d",
			value, n.Minimum,
		)
	}
	if value > n.Maximum {
		return singleIssue("", "Number %d at {path} is larger than maximum %d",
			value, n.Maximum,
		)
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (n Number) Map(data interface{}, target interface{}) error {
	if err := n.Validate(data); err != nil {
		return err
	}

	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		val.SetFloat(data.(float64))
		return nil
	default:
		return ErrTypeMismatch
	}
}

// Boolean schema type.
type Boolean struct{ MetaData }

// Schema returns a JSON representation of the schema.
func (b Boolean) Schema() map[string]interface{} {
	m := b.schema()
	m["type"] = typeBoolean
	return m
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (b Boolean) Validate(data interface{}) error {
	if _, ok := data.(bool); !ok {
		return singleIssue("", "Expected a boolean at {path}")
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (b Boolean) Map(data interface{}, target interface{}) error {
	if err := b.Validate(data); err != nil {
		return err
	}

	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	switch val.Kind() {
	case reflect.Bool:
		val.SetBool(data.(bool))
		return nil
	default:
		return ErrTypeMismatch
	}
}

// String schema type.
type String struct {
	MetaData
	MinimumLength int
	MaximumLength int
	Pattern       string
}

// Schema returns a JSON representation of the schema.
func (s String) Schema() map[string]interface{} {
	m := s.schema()
	m["type"] = typeString
	if s.MinimumLength != 0 {
		m["minLength"] = s.MinimumLength
	}
	if s.MaximumLength != 0 {
		m["maxLength"] = s.MaximumLength
	}
	if s.Pattern != "" {
		m["pattern"] = s.Pattern
	}
	return m
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (s String) Validate(data interface{}) error {
	value, ok := data.(string)
	if !ok {
		return singleIssue("", "Expected a string at {path}")
	}

	e := &ValidationError{}

	if s.MinimumLength != 0 && len(value) < s.MinimumLength {
		e.addIssue("",
			"String '%s' at {path} is shorter than minimum %d length allowed",
			value, s.MinimumLength)
	}
	if s.MaximumLength != 0 && len(value) > s.MaximumLength {
		e.addIssue("",
			"String '%s' at {path} is longer than maximum %d length allowed",
			value, s.MaximumLength)
	}
	if s.Pattern != "" {
		if match, _ := regexp.MatchString(s.Pattern, value); !match {
			e.addIssue("", "String '%s' doesn't match regular expression '%s'",
				value, s.Pattern)
		}
	}

	if len(e.issues) > 0 {
		return e
	}
	return nil
}

// Map takes data, validates and maps it into the target reference.
func (s String) Map(data interface{}, target interface{}) error {
	if err := s.Validate(data); err != nil {
		return err
	}

	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	switch val.Kind() {
	case reflect.String:
		val.SetString(data.(string))
		return nil
	default:
		return ErrTypeMismatch
	}
}

// StringEnum schema type for enums of strings.
type StringEnum struct {
	MetaData
	Options []string
}

// Schema returns a JSON representation of the schema.
func (s StringEnum) Schema() map[string]interface{} {
	m := s.schema()
	m["type"] = typeString
	m["enum"] = s.Options
	return m
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (s StringEnum) Validate(data interface{}) error {
	value, ok := data.(string)
	if !ok {
		return singleIssue("", "Expected a string at {path}")
	}

	if !stringContains(s.Options, value) {
		e := &ValidationError{}
		e.addIssue("",
			"Value '%s' at {path} is not valid for the enum with options: %v",
			value, s.Options)
		return e
	}

	return nil
}

// Map takes data, validates and maps it into the target reference.
func (s StringEnum) Map(data interface{}, target interface{}) error {
	if err := s.Validate(data); err != nil {
		return err
	}

	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	switch val.Kind() {
	case reflect.String:
		val.SetString(data.(string))
		return nil
	default:
		return ErrTypeMismatch
	}
}

// URI schema type for strings with format: uri.
type URI struct {
	MetaData
	Options []string
}

// Schema returns a JSON representation of the schema.
func (s URI) Schema() map[string]interface{} {
	m := s.schema()
	m["type"] = typeString
	m["format"] = "uri"
	return m
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (s URI) Validate(data interface{}) error {
	value, ok := data.(string)
	if !ok {
		return singleIssue("", "Expected a string at {path}")
	}

	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" {
		e := &ValidationError{}
		e.addIssue("", "Value '%s' at {path} is not a valid URI", value)
		return e
	}

	return nil
}

var typeOfURL = reflect.TypeOf((*url.URL)(nil)).Elem()

// Map takes data, validates and maps it into the target reference.
func (s URI) Map(data interface{}, target interface{}) error {
	if err := s.Validate(data); err != nil {
		return err
	}

	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	switch val.Kind() {
	case reflect.String:
		val.SetString(data.(string))
		return nil
	case reflect.Ptr:
		if val.Type().Elem() != typeOfURL {
			return ErrTypeMismatch
		}
		u, _ := url.Parse(data.(string))
		val.Set(reflect.ValueOf(u))
		return nil
	case reflect.Struct:
		if val.Type() != typeOfURL {
			return ErrTypeMismatch
		}
		u, _ := url.Parse(data.(string))
		val.Set(reflect.ValueOf(*u))
		return nil
	default:
		return ErrTypeMismatch
	}
}

// DateTime schema type for strings with format: date-time.
type DateTime struct {
	MetaData
}

// Schema returns a JSON representation of the schema.
func (d DateTime) Schema() map[string]interface{} {
	m := d.schema()
	m["type"] = typeString
	m["format"] = "date-time"
	return m
}

func parseDateTime(input string) (time.Time, error) {
	if result, err := time.Parse(time.RFC3339Nano, input); err == nil {
		return result, nil
	}
	return time.Parse(time.RFC3339, input)
}

// Validate the given data, this will return nil if data satisfies this schema.
// Otherwise, Validate(data) returns a ValidationError instance.
func (d DateTime) Validate(data interface{}) error {
	value, ok := data.(string)
	if !ok {
		return singleIssue("", "Expected a string at {path}")
	}

	// Try to parse date
	if _, err := parseDateTime(value); err != nil {
		e := &ValidationError{}
		e.addIssue("", "Value '%s' at {path} is not a valid date-time string", value)
		return e
	}

	return nil
}

var typeOfTime = reflect.TypeOf((*time.Time)(nil)).Elem()

// Map takes data, validates and maps it into the target reference.
func (d DateTime) Map(data interface{}, target interface{}) error {
	if err := d.Validate(data); err != nil {
		return err
	}

	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr {
		return ErrTypeMismatch
	}
	val := ptr.Elem()

	switch val.Kind() {
	case reflect.String:
		val.SetString(data.(string))
		return nil
	case reflect.Ptr:
		if val.Type().Elem() != typeOfTime {
			return ErrTypeMismatch
		}
		t, _ := parseDateTime(data.(string))
		val.Set(reflect.ValueOf(&t))
		return nil
	case reflect.Struct:
		if val.Type() != typeOfTime {
			return ErrTypeMismatch
		}
		t, _ := parseDateTime(data.(string))
		val.Set(reflect.ValueOf(t))
		return nil
	default:
		return ErrTypeMismatch
	}
}
