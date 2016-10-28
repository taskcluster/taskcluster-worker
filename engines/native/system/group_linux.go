package system

import (
	"fmt"
	"os/user"
	"strconv"
)

// Group is a representation of a system user-group.
type Group struct {
	gid int
}

// FindGroup will find the user-group given by name.
func FindGroup(name string) (*Group, error) {
	g, err := user.LookupGroup(name)
	if err != nil {
		if _, ok := err.(user.UnknownGroupError); ok {
			return nil, ErrUserGroupNotFound
		}
		return nil, fmt.Errorf("Failed to lookup user-group, error: %s", err)
	}
	// Parse group id, which should always be an integer on POSIX systems.
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		panic(fmt.Sprintf(
			"gid: '%s' for group: %s should have been an integer on POSIX system",
			g.Gid, name,
		))
	}
	return &Group{gid}, nil
}
