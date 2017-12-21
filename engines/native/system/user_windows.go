package system

import (
	"os/user"

	"github.com/pkg/errors"
)

// User is a representation of a system user account.
type User struct {
	name       string
	homeFolder string
}

// CurrentUser will get a User record representing the current user.
func CurrentUser() (*User, error) {
	u, err := user.Current()
	if err != nil {
		return nil, errors.Wrap(err, "failed to lookup current user")
	}
	return &User{
		name:       u.Username,
		homeFolder: u.HomeDir,
	}, nil
}

// FindUser will get a User record representing the user with given username.
func FindUser(username string) (*User, error) {
	panic("Not implemented")
}

// CreateUser will create a new user, with the given homeFolder, set the user
// owner of the homeFolder, and assign the user membership of given groups.
func CreateUser(homeFolder string, groups []*Group) (*User, error) {
	panic("Not implemented")
}

// Remove will remove a user and all associated resources.
func (u *User) Remove() {
	// Kill all process owned by this user, for good measure
	_ = KillByOwner(u)

	panic("Not implemented")
}

// Name returns the user name
func (u *User) Name() string {
	return u.name
}

// Home returns the user home folder`
func (u *User) Home() string {
	return u.homeFolder
}
