package runtime

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCreateLogger(t *testing.T) {
	logger, err := CreateLogger("debug")
	assert.Equal(t, err, nil, fmt.Sprintf("Error should not have been returned. %s", err))
	assert.Equal(t, reflect.TypeOf(logger).String(), "*logrus.Logger")
	assert.Equal(t, logger.Level, logrus.DebugLevel)
}

func TestLoggerNotCreatedWithInvalidLevel(t *testing.T) {
	_, err := CreateLogger("debug1234")
	assert.NotEqual(t, err, nil, fmt.Sprintf("Error should not have been returned. %s", err))
}

func TestDefaultWarnLevel(t *testing.T) {
	logger, err := CreateLogger("")
	assert.Equal(t, err, nil, fmt.Sprintf("Error should not have been returned. %s", err))
	assert.Equal(t, reflect.TypeOf(logger).String(), "*logrus.Logger")
	assert.Equal(t, logger.Level, logrus.WarnLevel)
}
