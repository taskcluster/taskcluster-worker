package log

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

type loggingLevel int

// Log messages can have one of 3 possible states.
const (
	DEBUG loggingLevel = 1 << iota
	INFO
	CRITICAL
)

// List of fields that should not be overwritten by fields specified when logging
// a message
var restrictedFields = []string{"level", "time", "message"}

// Creates a new logging instance
func NewLogger(out io.Writer, level loggingLevel, fields map[string]interface{}) *logger {
	return &logger{
		out:           out,
		defaultFields: fields,
		level:         level,
	}
}

type logger struct {
	// Create a lock when writing to `out` so output is not intermingled
	mu  sync.Mutex
	out io.Writer
	// List of default fields that will be serialized
	// along with the message and written to `out`
	defaultFields map[string]interface{}
	level         loggingLevel
}

// Log a debug message as long as the logger's debug level is set to at least Debug
func (l *logger) Debug(message string, fields map[string]interface{}) {
	if l.level > DEBUG {
		return
	}
	f := l.createMessage(message, fields)
	f["level"] = "debug"
	l.Write(f)
}

// Log an informational message as long as the logger's debug level is set to at least Info
func (l *logger) Info(message string, fields map[string]interface{}) {
	if l.level > INFO {
		return
	}
	f := l.createMessage(message, fields)
	f["level"] = "info"
	l.Write(f)
}

// Log a critical message.  Critical Messages will have "[alert-operator]" prepended to them
// for alerting purposes.
func (l *logger) Critical(message string, fields map[string]interface{}) {
	f := l.createMessage(message, fields)
	f["level"] = "critical"
	f["message"] = fmt.Sprintf("[alert-operator] %s", f["message"])
	l.Write(f)
}
func (l *logger) Write(message map[string]interface{}) {
	// ignore if there is an error, logging failure should not cause something fatal
	if output, err := json.Marshal(message); err == nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		if _, err := l.out.Write(output); err != nil {
			fmt.Println("Error writing to out")
		}
		if _, err := l.out.Write([]byte("\n")); err != nil {
			fmt.Println("Error writing to out")
		}
	}
}

// Creates a map with the default fields along with any fields that were added at
// the time of logging the message.
func (l *logger) createMessage(message string, fields map[string]interface{}) map[string]interface{} {
	f := map[string]interface{}{
		"message": message,
		"time":    time.Now().Unix(),
	}
	for k, v := range fields {
		fieldName := k
		if contains(restrictedFields, k) {
			fieldName = fmt.Sprintf("field_%s", k)
		}
		f[fieldName] = v
	}

	for k, v := range l.defaultFields {
		f[k] = v
	}

	return f
}

func contains(list []string, value interface{}) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
