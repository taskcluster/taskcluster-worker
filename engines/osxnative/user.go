// +build darwin

package osxnative

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/taskcluster/slugid-go/slugid"
)

type user struct {
	d                   dscl
	name                string
	supplementaryGroups []string
}

func newUser(sudo bool) user {
	return user{
		d: dscl{
			sudo: sudo,
		},
	}
}

func (u *user) create(groups []string) error {
	var err error
	defer func() {
		if err != nil {
			u.delete()
		}
	}()

	uid, err := u.getMaxUID()
	if err != nil {
		return err
	}

	name := "worker-" + slugid.Nice()
	userPath := path.Join("/Users", name)

	err = u.d.create(userPath)
	if err != nil {
		return err
	}

	// We set the uid, then check if it is unique, if
	// not, increment and try again
	duplicated := true
	for duplicated {
		uid++
		strUID := strconv.Itoa(uid)

		// set user uid
		err = u.d.create(userPath, "uid", strUID)
		if err != nil {
			return err
		}

		// check if uid has been used for another user
		duplicated, err = u.isDuplicatedID("uid", uid)
		if err != nil {
			return err
		}
	}

	if len(groups) > 0 {
		var primaryGroupID string
		primaryGroupID, err = u.d.read("/Groups/"+groups[0], "PrimaryGroupID")
		if err != nil {
			return err
		}

		err = u.d.create(userPath, "PrimaryGroupID", primaryGroupID)
		if err != nil {
			return err
		}

		for _, group := range groups[1:] {
			if err = u.d.append("/Groups/"+group, "GroupMembership", name); err != nil {
				return err
			}
			u.supplementaryGroups = append(u.supplementaryGroups, group)
		}
	}

	err = u.d.create(userPath, "NFSHomeDirectory", userPath)
	if err != nil {
		return err
	}

	err = os.MkdirAll(userPath, 0700)
	if err != nil {
		return err
	}

	err = os.Chown(userPath, uid, 0)
	if err != nil {
		return err
	}

	u.name = name

	return nil
}

func (u *user) delete() error {
	if u.name == "" {
		return nil
	}

	var errList []error

	for _, group := range u.supplementaryGroups {
		if err := u.d.delete("/Groups/"+group, "GroupMembership", u.name); err != nil {
			errList = append(errList, fmt.Errorf("Could not remove user %s from group %s: %v", u.name, group, err))
		}
	}

	userPath := path.Join("/Users", u.name)

	if err := u.d.delete(userPath); err != nil {
		os.RemoveAll(userPath)
		errList = append(errList, fmt.Errorf("Could not remove user %s: %v", u.name, err))
	}

	if err := os.RemoveAll(userPath); err != nil {
		errList = append(errList, fmt.Errorf("Could not remove user home directory %s: %v", u.name, err))
	}

	u.name = ""

	if len(errList) > 0 {
		return errList[0]
	}

	return nil
}

// return the next uid available
func (u *user) getMaxUID() (int, error) {
	uids, err := u.d.list("/Users", "uid")

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

func (u *user) isDuplicatedID(name string, id int) (bool, error) {
	entries, err := u.d.list("/Users", name)

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
