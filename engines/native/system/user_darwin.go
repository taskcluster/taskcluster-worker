package system

import (
	"fmt"
	"os/exec"
	"os/user"
	"path"
	"strconv"

	"github.com/taskcluster/slugid-go/slugid"
)

const defaultShell = "/bin/bash"

// User is a representation of a system user account.
type User struct {
	uid        uint32 // user id
	gid        uint32 // primary group id
	name       string
	homeFolder string
	groups     []string // additional user groups
}

// CurrentUser will get a User record representing the current user.
func CurrentUser() (*User, error) {
	osUser, err := user.Current()
	if err != nil {
		return nil, err
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

	return &User{
		uid:        uint32(uid),
		gid:        uint32(gid),
		name:       osUser.Username,
		homeFolder: osUser.HomeDir,
	}, nil
}

// FindUser will get a User record representing the user with given username.
func FindUser(username string) (*User, error) {
	panic("Not implemented")
}

// CreateUser will create a new user, with the given homeFolder, set the user
// owner of the homeFolder, and assign the user membership of given groups.
func CreateUser(homeFolder string, groups []*Group) (*User, error) {
	d := dscl{
		sudo: false,
	}

	// Generate a random username
	name := "worker-" + slugid.Nice()
	userPath := path.Join("/Users", name)

	err := d.create(userPath)
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf(
				"Failed to create user with useradd, stderr: '%s'", string(e.Stderr),
			)
		}
		return nil, fmt.Errorf("Failed to run useradd, error: %s", err)
	}

	newUID, err := getMaxUID(d)
	if err != nil {
		panic(fmt.Errorf("Error trying to figure out the max UID: %v", err))
	}

	// We set the uid, then check if it is unique, if
	// not, increment and try again
	duplicated := true
	for duplicated {
		newUID++
		strUID := strconv.Itoa(newUID)

		// set user uid
		err = d.create(userPath, "uid", strUID)
		if err != nil {
			panic(fmt.Errorf("Could not create user '%s': %v", userPath, err))
		}

		// check if uid has been used for another user
		duplicated, err = isDuplicateID(d, "uid", newUID)
		if err != nil {
			panic(fmt.Errorf("Error checking if new uid %d already exists: %v", newUID, err))
		}
	}

	groupName := path.Join("/Groups", name)
	err = d.create(groupName)
	if err != nil {
		panic(fmt.Errorf("Could not create group '%s': %v", groupName, err))
	}

	err = d.create(groupName, "gid", strconv.Itoa(newUID))
	if err != nil {
		panic(fmt.Errorf("Error set gid for the group '%s': %v", groupName, err))
	}

	g, err := user.LookupGroup(name)
	if err != nil {
		panic(fmt.Errorf("Error looking for group '%s': %v", name, err))
	}

	gid, err := strconv.ParseUint(g.Gid, 10, 32)
	if err != nil {
		panic(err)
	}
	if err = d.create(userPath, "PrimaryGroupID", g.Gid); err != nil {
		panic(fmt.Errorf("Error setting primary group id: %v", err))
	}

	supplementaryGroups := []string{}
	for _, group := range groups {
		g, err = user.LookupGroupId(strconv.Itoa(group.gid))
		if err != nil {
			panic(err)
		}
		supplementaryGroups = append(supplementaryGroups, g.Name)
		if err = d.append("/Groups/"+g.Name, "GroupMembership", name); err != nil {
			panic(err)
		}
	}

	err = d.create(userPath, "NFSHomeDirectory", homeFolder)
	if err != nil {
		panic(fmt.Errorf("Error setting home directory: %v", err))
	}

	if err = chownR(homeFolder, newUID, int(gid)); err != nil {
		panic(fmt.Errorf("Could not change owner of '%s' to user %v: %v", homeFolder, newUID, err))
	}

	return &User{
		uid:        uint32(newUID),
		gid:        uint32(gid),
		name:       name,
		homeFolder: homeFolder,
		groups:     supplementaryGroups,
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

	d := dscl{
		sudo: false,
	}
	// Kill all process owned by this user, for good measure
	_ = KillByOwner(u)

	for _, group := range u.groups {
		err := d.delete("/Groups/"+group, "GroupMembership", u.name)
		if err != nil {
			panic(err)
		}
	}

	// delete primary group
	if err := d.delete(path.Join("/Groups", u.name)); err != nil {
		panic(err)
	}

	if err := d.delete(path.Join("/Users", u.name)); err != nil {
		panic(err)
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

// return the next uid available
func getMaxUID(d dscl) (int, error) {
	uids, err := d.list("/Users", "uid")

	if err != nil {
		return -1, err
	}

	maxUID := 0
	for _, entry := range uids {
		uid, err := strconv.Atoi(entry[1])

		if err != nil {
			return -1, err
		}

		if uid > maxUID {
			maxUID = uid
		}
	}

	return maxUID, nil
}

func isDuplicateID(d dscl, name string, id int) (bool, error) {
	entries, err := d.list("/Users", name)

	if err != nil {
		return true, err
	}

	n := 0
	for _, entry := range entries {
		entryID, err := strconv.Atoi(entry[1])
		if err != nil {
			return true, err
		}

		if id == entryID {
			n++
		}
	}

	return n > 1, nil
}
