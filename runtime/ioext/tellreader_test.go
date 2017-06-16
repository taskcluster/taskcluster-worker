package ioext

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTellReader(t *testing.T) {
	r := TellReader{Reader: bytes.NewBufferString("hello world")}
	r.Read(make([]byte, 5))
	assert.EqualValues(t, 5, r.Tell())
	io.Copy(ioutil.Discard, &r)
	assert.EqualValues(t, 11, r.Tell())
}
