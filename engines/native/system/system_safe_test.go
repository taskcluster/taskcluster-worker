package system

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	rt "runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// This file contains test cases that don't leave risk leaving dirty system
// resources, such as temporary user accounts. However, this file doesn't
// test all the functionality in the system package. Useful, we you want to
// check some of the system package without risking a dirty system.

// Note: Constants testGroup, testCat, testTrue, testFalse, testPrintDir, and
//       testSleep, testChildren, testGroups should be defined per platform

func TestSystemSafely(t *testing.T) {
	var err error
	var u *User

	// Setup temporary home directory
	homeDir := filepath.Join(os.TempDir(), slugid.Nice())
	require.NoError(t, os.MkdirAll(homeDir, 0777))
	defer os.RemoveAll(homeDir)

	t.Run("CurrentUser", func(t *testing.T) {
		u, err = CurrentUser()
		assert.NoError(t, err)
		assert.NotEmpty(t, u.Name())
	})

	t.Run("FindGroup", func(t *testing.T) {
		t.Skip("Not implemented on windows yet")
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
		t.Skip("Not implemented on windows, we're missing a testCat command that works")
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
		t.Skip("Not implemented on windows, we're missing a testCat command that works")
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
		if rt.GOOS == "darwin" {
			t.Skip("TODO: fix test can on OS X, no idea why it fails")
		}
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
		require.Contains(t, out.String(), u.Home())
	})

	t.Run("StartProcess TTY, Owner and Print Dir", func(t *testing.T) {
		if rt.GOOS == "darwin" {
			t.Skip("TODO: fix test can on OS X, no idea why it fails")
		}
		var out bytes.Buffer
		p, err := StartProcess(ProcessOptions{
			Arguments: testPrintDir,
			Stdout:    ioext.WriteNopCloser(&out),
			Owner:     u,
			TTY:       true,
		})
		require.NoError(t, err)
		require.True(t, p.Wait())
		require.Contains(t, out.String(), u.Home())
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

	t.Run("KillProcessTree", func(t *testing.T) {
		p, err := StartProcess(ProcessOptions{
			Arguments: testChildren,
			Owner:     nil,
		})
		require.NoError(t, err)
		done := make(chan bool)
		go func() {
			done <- p.Wait()
		}()
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, KillProcessTree(p))
		select {
		case result := <-done:
			require.False(t, result, "killed process should result in false")
		case <-time.After(10 * time.Second):
			t.Fatal("Processes weren't killed")
		}
	})
}
