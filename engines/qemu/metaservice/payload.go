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
	ErrorCodeUnknownActionID  = "UnknownActionId"
	ErrorCodeInvalidPayload   = "InvalidPayload"
)

// Error is the response payload for any error senario.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Action is the response payload for the /engine/v1/poll end-point.
type Action struct {
	ID      string   `json:"id"`      // id, to be used when replying
	Type    string   `json:"type"`    // none, get-artifact, list-folder, exec-shell
	Path    string   `json:"path"`    // file path, if get-artifact/list-folder
	Command []string `json:"command"` // Command for exec-shell
	TTY     bool     `json:"tty"`     // TTY or not for exec-shell
}

// Files is the request payload for the /engine/v1/list-folder end-point.
type Files struct {
	Files    []string `json:"files"`    // List of absolute file paths
	NotFound bool     `json:"notFound"` // true, if path doesn't exist
}
