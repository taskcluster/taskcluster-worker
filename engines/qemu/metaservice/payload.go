package metaservice

// Execute is the response payload for the /engine/v1/execute end-point.
type Execute struct {
	Env     map[string]string `json:"env"`
	Command []string          `json:"command"`
}

// List of API error codes for using the Error struct.
const (
	ErrorCodeMethodNotAllowed = "MethodNotAllowed"
	ErrorCodeNoSuchEndPoint   = "NoSuchEndPoint"
	ErrorCodeInternalError    = "InternalError"
	ErrorCodeResourceConflict = "ResourceConflict"
)

// Error is the response payload for any error senario.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
