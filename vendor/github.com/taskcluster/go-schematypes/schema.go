package schematypes

// A Schema is implemented by any object that can represent a JSON schema.
type Schema interface {
	Schema() map[string]interface{}
	Validate(data interface{}) error
	Map(data, target interface{}) error
}
