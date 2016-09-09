package schematypes

// A Schema is implemented by any object that can represent a JSON schema.
type Schema interface {
	Schema() map[string]interface{}
	Validate(data interface{}) error
	Map(data, target interface{}) error
}

// An AnyOf instance represents the anyOf JSON schema construction.
type AnyOf []Schema

// A OneOf instance represents the oneOf JSON schema construction.
type OneOf []Schema

// An AllOf instance represents the allOf JSON schema construction.
type AllOf []Schema
