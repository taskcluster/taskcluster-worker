// +build darwin,system

package system

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
)

func hasString(x string, y []string) bool {
	for _, s := range y {
		if s == x {
			return true
		}
	}

	return false
}

func TestDarwinSystem(t *testing.T) {
	t.Run("CreateUserDarwin", func(t *testing.T) {
		var err error
		var u *User

		// Setup temporary home directory
		homeDir := filepath.Join(os.TempDir(), slugid.Nice())
		require.NoError(t, os.MkdirAll(homeDir, 0777))
		defer os.RemoveAll(homeDir)

		groups := []*Group{}
		for _, groupName := range testGroups {
			var g *Group

			g, err = FindGroup(groupName)
			require.NoError(t, err)

			require.NoError(t, err)

			groups = append(groups, g)
		}

		u, err = CreateUser(homeDir, groups)
		require.NoError(t, err)
		defer u.Remove()

		fileInfo, err := os.Stat(homeDir)
		require.NoError(t, err)
		require.Equal(t, u.uid, fileInfo.Sys().(*syscall.Stat_t).Uid)

		su, err := user.Lookup(u.name)
		require.NoError(t, err)

		// some sanity checks
		require.EqualValues(t, u.name, su.Username)

		uid, err := strconv.ParseUint(su.Uid, 10, 32)
		require.NoError(t, err)
		require.EqualValues(t, u.uid, uint32(uid))

		gid, err := strconv.ParseUint(su.Gid, 10, 32)
		require.NoError(t, err)
		require.EqualValues(t, u.gid, gid)

		g, err := user.LookupGroup(u.name)
		require.NoError(t, err)

		gid, err = strconv.ParseUint(g.Gid, 10, 32)
		require.NoError(t, err)
		require.EqualValues(t, u.gid, uint32(gid))

		gids, err := su.GroupIds()
		require.NoError(t, err)

		for _, group := range testGroups {
			g, err := FindGroup(group)
			require.NoError(t, err)
			gid := strconv.Itoa(g.gid)
			require.NoError(t, err)
			require.True(t, hasString(gid, gids))
		}
	})
}
