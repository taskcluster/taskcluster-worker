package system

import "errors"

// ErrUserGroupNotFound indicates that a given user-group doesn't exist.
var ErrUserGroupNotFound = errors.New("User group doesn't exist!")
