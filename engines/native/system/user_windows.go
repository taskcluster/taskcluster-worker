package system

// User is a representation of a system user account.
type User struct {
	name       string
	homeFolder string
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
