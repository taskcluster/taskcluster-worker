package log

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLogger(t *testing.T) {
	logger := NewLogger(os.Stdout, DEBUG, nil)
	assert.Equal(t, reflect.TypeOf(logger).String(), "*log.logger", "Logger was not created")
}
func TestDebugLogLevel(t *testing.T) {
	type testLogMessage struct {
		Time    int    `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}

	var (
		b           bytes.Buffer
		testMessage testLogMessage
	)
	logger := NewLogger(&b, DEBUG, nil)
	logger.Debug("testing debug level", nil)
	err := json.Unmarshal([]byte(b.String()), &testMessage)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, testMessage.Level, "debug")
	assert.Equal(t, testMessage.Message, "testing debug level")
}

func TestInfoLogLevel(t *testing.T) {
	type testLogMessage struct {
		Time    int    `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}

	var (
		b           bytes.Buffer
		testMessage testLogMessage
	)
	logger := NewLogger(&b, DEBUG, nil)
	logger.Info("testing info level", nil)
	err := json.Unmarshal([]byte(b.String()), &testMessage)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, testMessage.Level, "info")
	assert.Equal(t, testMessage.Message, "testing info level")
}

func TestMessageNotLoggedIfLevelNotHighEnough(t *testing.T) {
	type testLogMessage struct {
		Time    int    `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}

	var b bytes.Buffer
	logger := NewLogger(&b, INFO, nil)
	logger.Debug("testing debug level", nil)
	assert.Equal(t, b.String(), "", "No message should have been written")
}

func TestDefaultFields(t *testing.T) {
	type testLogMessage struct {
		Time    int    `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
		Worker  string `json:"worker"`
		Region  string `json:"region"`
	}

	var (
		b           bytes.Buffer
		testMessage testLogMessage
	)
	logger := NewLogger(&b, DEBUG, map[string]interface{}{"worker": "foobar", "region": "central"})
	logger.Debug("testing", nil)
	err := json.Unmarshal([]byte(b.String()), &testMessage)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, testMessage.Region, "central")
	assert.Equal(t, testMessage.Worker, "foobar")
}

func TestFieldsWithMessage(t *testing.T) {
	type testLogMessage struct {
		Time    int    `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
		Worker  string `json:"worker"`
		Region  string `json:"region"`
		Testing int    `json:"testing"`
	}

	var (
		b           bytes.Buffer
		testMessage testLogMessage
	)
	logger := NewLogger(&b, DEBUG, map[string]interface{}{"worker": "foobar", "region": "central"})
	logger.Debug("testing", map[string]interface{}{"testing": 1234})
	err := json.Unmarshal([]byte(b.String()), &testMessage)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, testMessage.Region, "central")
	assert.Equal(t, testMessage.Worker, "foobar")
	assert.Equal(t, testMessage.Testing, 1234)
}

func TestRestrictedFieldNotOverriden(t *testing.T) {
	type testLogMessage struct {
		Time       int    `json:"time"`
		Level      string `json:"level"`
		Message    string `json:"message"`
		FieldLevel int    `json:"field_level"`
	}

	var (
		b           bytes.Buffer
		testMessage testLogMessage
	)
	logger := NewLogger(&b, DEBUG, nil)
	logger.Info("testing ", map[string]interface{}{"level": 1234})
	err := json.Unmarshal([]byte(b.String()), &testMessage)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, testMessage.Level, "info")
	assert.Equal(t, testMessage.FieldLevel, 1234)
}

func TestCriticalMessageContainsAlertPrefix(t *testing.T) {
	type testLogMessage struct {
		Time    int    `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}

	var (
		b           bytes.Buffer
		testMessage testLogMessage
	)
	logger := NewLogger(&b, DEBUG, nil)
	logger.Critical("something bad happened", nil)
	err := json.Unmarshal([]byte(b.String()), &testMessage)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, testMessage.Level, "critical")
	assert.Equal(t, testMessage.Message, "[alert-operator] something bad happened")
}
