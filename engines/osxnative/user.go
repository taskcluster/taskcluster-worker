// +build darwin

package osxnative

import (
	"github.com/taskcluster/slugid-go/slugid"
	"os"
	"path"
	"strconv"
)

type user struct {
	d    dscl
	name string
}

func (u *user) create() error {
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

	staffGid, err := u.d.read("/Groups/staff", "PrimaryGroupID")
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

	err = u.d.create(userPath, "PrimaryGroupID", staffGid)
	if err != nil {
		return err
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

	userPath := path.Join("/Users", u.name)
	err := u.d.delete(userPath)
	if err != nil {
		return err
	}

	err = os.RemoveAll(userPath)
	u.name = ""
	return err
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
