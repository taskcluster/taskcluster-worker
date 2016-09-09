package schematypes

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTypeMismatch is returned when trying to map to a type that doesn't match
// or which isn't writable (for example passed by value and not pointer).
var ErrTypeMismatch = errors.New("Type does not match the schema")

// A ValidationIssue is any error found validating a JSON object.
type ValidationIssue struct {
	message string
	path    string
}

// Prefix will add a prefix to the path to property that had an issue.
func (v *ValidationIssue) Prefix(prefix string, args ...interface{}) ValidationIssue {
	return ValidationIssue{
		message: v.message,
		path:    fmt.Sprintf(prefix, args...) + v.path,
	}
}

// ValidationError represents a validation failure as a list of validation
// issues.
type ValidationError struct {
	issues []ValidationIssue
}

// Issues returns the validation issues
func (e *ValidationError) Issues() []ValidationIssue {
	return e.issues
}

func (e *ValidationError) addIssue(path, message string, args ...interface{}) {
	e.issues = append(e.issues, ValidationIssue{
		message: fmt.Sprintf(message, args...),
		path:    path,
	})
}

func (e *ValidationError) addIssues(err *ValidationError) {
	e.issues = append(e.issues, err.issues...)
}

func (e *ValidationError) addIssuesWithPrefix(err error, prefix string, args ...interface{}) {
	if err == nil {
		return
	}
	if err, ok := err.(*ValidationError); ok {
		for _, issue := range err.issues {
			e.issues = append(e.issues, issue.Prefix(prefix, args...))
		}
	} else {
		issue := ValidationIssue{
			message: fmt.Sprintf("Error: %s at {path}", err.Error()),
			path:    "",
		}
		e.issues = append(e.issues, issue.Prefix(prefix, args...))
	}
}

func singleIssue(path, message string, args ...interface{}) *ValidationError {
	e := &ValidationError{}
	e.addIssue(path, message, args...)
	return e
}

func (e *ValidationError) Error() string {
	msg := "Validation error: "
	for _, issue := range e.issues {
		msg += strings.Replace(issue.message, "{path}", "root"+issue.path, -1) + ", "
	}
	return msg
}
