// +build darwin

package osxnative

import (
	assert "github.com/stretchr/testify/require"
	"os"
	osuser "os/user"
	"testing"
)

func TestUser(t *testing.T) {
	currentUser, err := osuser.Current()
	assert.NoError(t, err)

	if currentUser.Username != "root" {
		t.Skip("This test requires root privileges, skipping.")
	}

	u := user{}
	assert.NoError(t, u.create())

	defer u.delete()

	userInfo, err := osuser.Lookup(u.name)
	assert.NoError(t, err)
	assert.Equal(t, userInfo.Username, u.name)

	// Check for the existence of the home directory
	_, err = os.Stat(userInfo.HomeDir)
	assert.NoError(t, err)

	// we defer u.delete() to make sure it is called in case of
	// a failure, but double calling it won't hurt (of couse if
	// user.delete is not buggy)
	err = u.delete()
	assert.NoError(t, err)

	_, err = os.Stat(userInfo.HomeDir)
	assert.Error(t, err, "Home directory should not exist")
	assert.True(t, os.IsNotExist(err))
}
