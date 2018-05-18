// +build linux

package dockerengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateMountPoint(t *testing.T) {
	invalidMountPoints := []string{
		"",
		"/",
		"//",
		"/test",
		"test",
		"test/",
		"/test/a",
		"/test/./",
		"../",
		"/..",
		"/test/../",
		string([]byte{'/', 't', 0, 't', '/'}), // try with \0
		"/test\\something/",
	}
	validMountPoints := []string{
		"/test/",
		"/test/test/",
		"/tmp/",
		"/var/lib/docker/",
		"/mnt/my-folder/",
	}

	for _, mountPoint := range invalidMountPoints {
		t.Run(`"`+mountPoint+`"`, func(t *testing.T) {
			err := validateMountPoint(mountPoint)
			assert.Error(t, err, "expected '"+mountPoint+"' to be valid")
		})
	}
	for _, mountPoint := range validMountPoints {
		t.Run(`"`+mountPoint+`"`, func(t *testing.T) {
			err := validateMountPoint(mountPoint)
			assert.NoError(t, err, "expected '%s' to be valid", mountPoint)
		})
	}
}
