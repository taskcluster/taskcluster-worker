// +build linux,system darwin,system

package system

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// Note: Constants testGroup, testCat, testTrue, testFalse, testPrintDir, and
//       testSleep, testGroups should be defined per platform

func TestSystem(t *testing.T) {
	var err error
	var u *User

	// Setup temporary home directory
	homeDir := filepath.Join(os.TempDir(), slugid.Nice())
	require.NoError(t, os.MkdirAll(homeDir, 0777))
	defer os.RemoveAll(homeDir)

	t.Run("CreateUser", func(t *testing.T) {
		u, err = CreateUser(homeDir, nil)
		require.NoError(t, err)
	})

	t.Run("FindGroup", func(t *testing.T) {
		g, err := FindGroup(testGroup)
		require.NoError(t, err)
		require.NotNil(t, g)
	})

	t.Run("StartProcess True", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testTrue,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
	})

	t.Run("StartProcess False", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testFalse,
		})
		require.NoError(t, err)
		require.False(t, p.Wait())
	})

	t.Run("StartProcess True with TTY", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testTrue,
			TTY:       true,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
	})

	t.Run("StartProcess False with TTY", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testFalse,
			TTY:       true,
		})
		require.NoError(t, err)
		require.False(t, p.Wait())
	})

	t.Run("StartProcess Cat", func(t *testing.T) {
		var out bytes.Buffer
		p, err := StartProcess(ProcessOptions{
			Arguments: testCat,
			Stdin:     ioutil.NopCloser(bytes.NewBufferString("hello-world")),
			Stdout:    ioext.WriteNopCloser(&out),
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
		require.EqualValues(t, "hello-world", out.String())
	})

	t.Run("StartProcess Cat with TTY", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testCat,
			TTY:       true,
			Stdin:     ioutil.NopCloser(bytes.NewBufferString("hello-world")),
			// We can't reliably read output as we kill the process with stdin
			// is closed (EOF)
		})
		require.NoError(t, err)
		assert.False(t, p.Wait())
	})

	t.Run("StartProcess Print Dir", func(t *testing.T) {
		var out bytes.Buffer
		p, err := StartProcess(ProcessOptions{
			Arguments:     testPrintDir,
			Stdout:        ioext.WriteNopCloser(&out),
			WorkingFolder: homeDir,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
		require.Contains(t, out.String(), homeDir)
	})

	t.Run("StartProcess TTY Print Dir", func(t *testing.T) {
		var out bytes.Buffer
		p, err := StartProcess(ProcessOptions{
			Arguments:     testPrintDir,
			Stdout:        ioext.WriteNopCloser(&out),
			WorkingFolder: homeDir,
			TTY:           true,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
		require.Contains(t, out.String(), homeDir)
	})

	t.Run("StartProcess Owner and Print Dir", func(t *testing.T) {
		var out bytes.Buffer
		p, err := StartProcess(ProcessOptions{
			Arguments: testPrintDir,
			Stdout:    ioext.WriteNopCloser(&out),
			Owner:     u,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
		require.Contains(t, out.String(), homeDir)
	})

	t.Run("StartProcess TTY, Owner and Print Dir", func(t *testing.T) {
		var out bytes.Buffer
		p, err := StartProcess(ProcessOptions{
			Arguments: testPrintDir,
			Stdout:    ioext.WriteNopCloser(&out),
			Owner:     u,
			TTY:       true,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
		require.Contains(t, out.String(), homeDir)
	})

	t.Run("StartProcess Owner and True", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testTrue,
			Owner:     u,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
	})

	t.Run("StartProcess TTY, Owner and True", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testTrue,
			Owner:     u,
			TTY:       true,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
	})

	t.Run("StartProcess Owner and False", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testFalse,
			Owner:     u,
		})
		require.NoError(t, err)
		require.False(t, p.Wait())
	})

	t.Run("StartProcess TTY, Owner and False", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testFalse,
			Owner:     u,
			TTY:       true,
		})
		require.NoError(t, err)
		require.False(t, p.Wait())
	})

	t.Run("StartProcess Kill", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testSleep,
		})
		require.NoError(t, err)
		var waited atomics.Bool
		done := make(chan bool)
		go func() {
			p.Wait()
			done <- waited.Get()
		}()
		time.Sleep(100 * time.Millisecond)
		waited.Set(true)
		p.Kill()
		require.False(t, p.Wait())
		require.True(t, <-done, "p.Wait was done before p.Kill() was called!")
	})

	t.Run("KillByOwner", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testSleep,
			Owner:     u,
		})
		require.NoError(t, err)
		var waited atomics.Bool
		done := make(chan bool)
		go func() {
			p.Wait()
			done <- waited.Get()
		}()
		time.Sleep(100 * time.Millisecond)
		waited.Set(true)
		require.NoError(t, KillByOwner(u))
		require.True(t, <-done, "p.Wait was done before KillByOwner was called!")
	})

	t.Run("user.Remove", func(t *testing.T) {
		u.Remove()
	})
}
