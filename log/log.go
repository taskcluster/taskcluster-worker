package log

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type loggingLevel int

const (
	DEBUG loggingLevel = 1 << iota
	INFO
	CRITICAL
)

var restrictedFields = []string{"level", "time", "message"}

func NewLogger(out io.Writer, level loggingLevel, fields map[string]interface{}) *logger {
	return &logger{out, fields, level}
}

type logger struct {
	out io.Writer
	// List of default fields that will be serialized
	// along with the message and written to `out`
	defaultFields map[string]interface{}
	level         loggingLevel
}

func (l *logger) Debug(message string, fields map[string]interface{}) {
	if l.level > DEBUG {
		return
	}
	f := l.createMessage(message, fields)
	f["level"] = "debug"
	l.Write(f)
}

func (l *logger) Info(message string, fields map[string]interface{}) {
	if l.level > INFO {
		return
	}
	f := l.createMessage(message, fields)
	f["level"] = "info"
	l.Write(f)
}

func (l *logger) Critical(message string, fields map[string]interface{}) {
	f := l.createMessage(message, fields)
	f["level"] = "critical"
	f["message"] = fmt.Sprintf("[alert-operator] %s", f["message"])
	l.Write(f)
}
func (l *logger) Write(message map[string]interface{}) {
	// ignore if there is an error, logging failure should not cause something fatal
	if output, err := json.Marshal(message); err == nil {
		l.out.Write(output)
		l.out.Write([]byte("\n"))
	}
}

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
