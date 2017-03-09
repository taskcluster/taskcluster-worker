package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsMalformedPayloadError(t *testing.T) {
	var err error
	err = NewMalformedPayloadError("test")
	_, ok := IsMalformedPayloadError(err)
	assert.True(t, ok)
}
