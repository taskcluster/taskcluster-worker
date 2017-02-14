package runtime

import "fmt"

// An ExceptionReason specifies the reason a task reached an exception state.
type ExceptionReason int

// Reasons why a task can reach an exception state. Implementors should be
// warned that additional entries may be added in the future.
const (
	ReasonNoException ExceptionReason = iota
	ReasonCanceled
	ReasonWorkerShutdown
	ReasonMalformedPayload
	ReasonResourceUnavailable
	ReasonInternalError
	ReasonSuperseded
	ReasonIntermittentTask
)

// String returns a string repesentation of the ExceptionReason for use with the
// taskcluster-queue API.
func (e ExceptionReason) String() string {
	switch e {
	case ReasonNoException:
		panic("ExceptionReason: ReasonNoException has no valid API representation")
	case ReasonCanceled:
		panic("ExceptionReason: ReasonCanceled has no valid API representation")
	case ReasonWorkerShutdown:
		return "worker-shutdown"
	case ReasonMalformedPayload:
		return "malformed-payload"
	case ReasonResourceUnavailable:
		return "resource-unavailable"
	case ReasonInternalError:
		return "internal-error"
	case ReasonSuperseded:
		return "superceded"
	case ReasonIntermittentTask:
		return "intermittent-task"
	}
	panic(fmt.Sprintf("Unknown ExceptionReason: %d", e))
}
