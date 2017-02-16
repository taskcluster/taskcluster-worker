// +build !windows,!plan9,!nacl

package system

import (
	"fmt"
	"os"
	osuser "os/user"
	"strconv"
)

// ChangeOwner changes the owner of filepath to the given user
func ChangeOwner(filepath string, user *User) error {
	u, err := osuser.Lookup(user.Name())
	if err != nil {
		return fmt.Errorf("Cannot lookup user %s: %v", user.Name(), err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("Cannot convert uid(%s) to int: %v", u.Uid, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("Cannot convert gid(%s) to int: %v", u.Gid, err)
	}
	if err = os.Chown(filepath, uid, gid); err != nil {
		return fmt.Errorf("Can't change owner of %s: %v", filepath, err)
	}

	return nil
}
