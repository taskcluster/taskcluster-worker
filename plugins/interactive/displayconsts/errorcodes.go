package displayconsts

const (
	// ErrorCodeDisplayNotFound signals that the given display couldn't be found
	ErrorCodeDisplayNotFound = "DisplayNotFound"
	// ErrorCodeExecutionTerminated indicates that the sandbox terminated, hence,
	// interactive displays can't be opened anymore
	ErrorCodeExecutionTerminated = "ExecutionTerminated"
	// ErrorCodeInternalError indicates some internal error, likely a connection
	// error or something.
	ErrorCodeInternalError = "InternalError"
	// ErrorCodeInvalidParameters indicates that the given display parameter isn't
	// valid, likely it's missing.
	ErrorCodeInvalidParameters = "InvalidParameters"
)
