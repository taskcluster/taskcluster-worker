package system

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/taskcluster/slugid-go/slugid"
)

const defaultShell = "/bin/bash"
const systemUserAdd = "/usr/sbin/useradd"
const systemUserDel = "/usr/sbin/userdel"

// User is a representation of a system user account.
type User struct {
	uid        uint32   // user id
	gid        uint32   // primary group id
	gids       []uint32 // users group memberships
	name       string
	homeFolder string
}

// CurrentUser will get a User record representing the current user.
func CurrentUser() (*User, error) {
	osUser, err := user.Current()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get current user")
	}

	// Uid and Gid are always decimal numbers on posix systems
	uid, err := strconv.Atoi(osUser.Uid)
	if err != nil {
		panic(fmt.Sprintf("Could not convert %s to integer: %s", osUser.Uid, err))
	}
	gid, err := strconv.Atoi(osUser.Gid)
	if err != nil {
		panic(fmt.Sprintf("Could not convert %s to integer: %s", osUser.Gid, err))
	}

	// Find group ids
	gids, err := findGroupIds(osUser)
	if err != nil {
		return nil, err
	}

	return &User{
		uid:        uint32(uid),
		gid:        uint32(gid),
		gids:       gids,
		name:       osUser.Username,
		homeFolder: osUser.HomeDir,
	}, nil
}

func findGroupIds(osUser *user.User) ([]uint32, error) {
	sgids, err := osUser.GroupIds()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list groups for current user")
	}
	gids := make([]uint32, len(sgids))
	for i, sgid := range sgids {
		igid, err := strconv.Atoi(sgid)
		if err != nil {
			panic(errors.Wrap(err, "Could not convert group id to integer"))
		}
		gids[i] = uint32(igid)
	}
	return gids, nil
}

// FindUser will get a User record representing the user with given username.
func FindUser(username string) (*User, error) {
	osUser, err := user.Lookup(username)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to lookup user: %s", username)
	}

	// Uid and Gid are always decimal numbers on posix systems
	uid, err := strconv.Atoi(osUser.Uid)
	if err != nil {
		panic(fmt.Sprintf("Could not convert %s to integer: %s", osUser.Uid, err))
	}
	gid, err := strconv.Atoi(osUser.Gid)
	if err != nil {
		panic(fmt.Sprintf("Could not convert %s to integer: %s", osUser.Gid, err))
	}

	// Find group ids
	gids, err := findGroupIds(osUser)
	if err != nil {
		return nil, err
	}

	return &User{
		uid:        uint32(uid),
		gid:        uint32(gid),
		gids:       gids,
		name:       osUser.Username,
		homeFolder: osUser.HomeDir,
	}, nil
}

// CreateUser will create a new user, with the given homeFolder, set the user
// owner of the homeFolder, and assign the user membership of given groups.
func CreateUser(homeFolder string, groups []*Group) (*User, error) {
	// Prepare arguments
	args := formatArgs(map[string]string{
		"-d": homeFolder,   // Set home folder
		"-c": "task user",  // Comment
		"-s": defaultShell, // Set default shell
	})
	args = append(args, "-M") // Don't create home, ignoring any global settings
	args = append(args, "-U") // Create primary user-group with same name
	if len(groups) > 0 {
		gids := []string{}
		for _, g := range groups {
			gids = append(gids, strconv.Itoa(g.gid))
		}
		args = append(args, "-G", strings.Join(gids, ","))
	}

	// Generate a random username
	name := slugid.Nice()
	args = append(args, name)

	// Run useradd command
	_, err := exec.Command(systemUserAdd, args...).Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf(
				"Failed to create user with useradd, stderr: '%s'", string(e.Stderr),
			)
		}
		return nil, fmt.Errorf("Failed to run useradd, error: %s", err)
	}

	// Lookup user to get the uid
	u, err := user.Lookup(name)
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to lookup newly created user: '%s', error: %s",
			name, err,
		))
	}

	// Parse uid/gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("user.Uid should be an integer on POSIX systems"))
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("user.Gid should be an integer on POSIX systems"))
	}
	debug("Created user with uid: %d, gid: %d, name: %s", uid, gid, name)

	// Set user as owner of home folder
	err = chownR(homeFolder, int(uid), int(gid))
	if err != nil {
		return nil, fmt.Errorf("Failed to chown homeFolder, error: %s", err)
	}

	// Find group ids
	osUser, err := user.Lookup(name)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to lookup user: %s", name)
	}
	gids, err := findGroupIds(osUser)
	if err != nil {
		return nil, err
	}

	return &User{
		uid:        uint32(uid),
		gid:        uint32(gid),
		gids:       gids,
		name:       name,
		homeFolder: homeFolder,
	}, nil
}

// Remove will remove a user and all associated resources.
func (u *User) Remove() {
	currentUser, err := CurrentUser()
	if err == nil {
		if currentUser.uid == u.uid {
			panic("oops, cannot delete current user " + u.Name())
		}
	}

	// Kill all process owned by this user, for good measure
	_ = KillByOwner(u)

	_, err = exec.Command(systemUserDel, u.name).Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			panic(fmt.Sprintf(
				"Failure removing user: %s (uid: %d), stderr: '%s'",
				u.name, u.uid, e.Stderr,
			))
		}
		panic(fmt.Sprintf(
			"Unable to remove user: %s (uid: %d), error: %s", u.name, u.uid, err,
		))
	}
}

// Name returns the user name
func (u *User) Name() string {
	return u.name
}

// Home returns the user home folder`
func (u *User) Home() string {
	return u.homeFolder
}
