package enginetest

// The EnvVarTestCase contains information sufficient to setting an environment
// variable.
type EnvVarTestCase struct {
	Engine string
	// Valid name for an environment variable.
	VariableName string
	// Invalid environment variable name.
	InvalidVariableName string
	// Payload that will print the value of VariableName to the log.
	Payload string
}
