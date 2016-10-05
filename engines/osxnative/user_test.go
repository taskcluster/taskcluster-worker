// +build darwin

package osxnative

import (
	"fmt"
	"os"
	osuser "os/user"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	currentUser, err := osuser.Current()
	assert.NoError(t, err)

	if currentUser.Username != "root" {
		t.Skip("This test requires root privileges, skipping.")
	}

	u := user{}
	groups := []string{"staff", "admin"}

	primaryGid, err := u.d.read("/Groups/"+groups[0], "PrimaryGroupID")
	assert.NoError(t, err)

	assert.NoError(t, u.create(groups))
	defer u.delete()

	primaryGroupID, err := u.d.read("/Users/"+u.name, "PrimaryGroupID")
	assert.NoError(t, err)
	assert.Equal(t, primaryGid, primaryGroupID)

	for _, group := range groups[1:] {
		var members string
		members, err = u.d.read("/Groups/"+group, "GroupMembership")
		fmt.Println(members)
		assert.True(t, strings.Contains(members, u.name))
	}

	userInfo, err := osuser.Lookup(u.name)
	assert.NoError(t, err)
	assert.Equal(t, userInfo.Username, u.name)

	// Check for the existence of the home directory
	_, err = os.Stat(userInfo.HomeDir)
	assert.NoError(t, err)

	// we defer u.delete() to make sure it is called in case of
	// a failure, but double calling it won't hurt (of course if
	// user.delete is not buggy)
	userName := u.name
	err = u.delete()
	assert.NoError(t, err)

	_, err = os.Stat(userInfo.HomeDir)
	assert.Error(t, err, "Home directory should not exist")
	assert.True(t, os.IsNotExist(err))

	_, err = u.d.read("/Users/" + userName)
	assert.Error(t, err)

	for _, group := range groups[1:] {
		var members string
		members, err = u.d.read("/Groups/"+group, "GroupMembership")
		assert.NoError(t, err)
		assert.False(t, strings.Contains(members, userName))
	}
}
