// +build qemu
// +build network

package openvpn

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

func fileAsJSON(t *testing.T, filename string) string {
	data, err := ioutil.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err, "Failed to read: %s", filename)

	raw, err := json.Marshal(string(data))
	require.NoError(t, err, "Failed to serialize JSON")
	return string(raw)
}

// Creating this file signals that the server has a connection
// we use this for testing... As the VPN object doesn't really track if we
// have connected the VPN-server or not. We can re-think that later, current
// semantics is to attempt to connect and retry forever.
const clientConnectedFile = "/tmp/test-vpn-server-has-client"

func TestOpenVPN(t *testing.T) {
	tmp := runtime.NewTemporaryTestFolderOrPanic()
	defer tmp.Remove()

	// Remove file if already there
	os.Remove(clientConnectedFile)
	defer os.Remove(clientConnectedFile) // clean up after test case

	// Monitor for clientConnectedFile to be created
	clientConnected := make(chan struct{})
	w, err := fsnotify.NewWatcher()
	require.NoError(t, err, "Failed to create file watcher")
	w.Add("/tmp")
	go func() {
		defer w.Close()
		for {
			select {
			case e := <-w.Events:
				if e.Op == fsnotify.Create && e.Name == clientConnectedFile {
					close(clientConnected)
					return
				}
			case werr := <-w.Errors:
				assert.NoError(t, werr, "file watcher failed")
				return
			}
		}
	}()

	// Setup a test vpn server
	server := exec.Command("openvpn", "test-server.ovpn")
	server.Dir = "testdata"
	err = server.Start()
	require.NoError(t, err, "Failed to start test server")
	defer server.Process.Kill()

	// Create VPN clients
	var config interface{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"remote": "localhost",
		"port": 16000,
		"cipher": "AES-256-CBC",
		"protocol": "udp",
		"routes": [],
		"tls": true,
		"certificateAuthority": `+fileAsJSON(t, "ca.crt")+`,
		"key": `+fileAsJSON(t, "client.key")+`,
		"certificate": `+fileAsJSON(t, "client.crt")+`,
		"tlsKey": `+fileAsJSON(t, "ta.key")+`,
		"keyDirection": 1,
		"renegotiationDelay": 10
	}`), &config))

	vpn, err := New(Options{
		DeviceName:       "tuntestclient",
		Config:           config,
		Monitor:          mocks.NewMockMonitor(false),
		TemporaryStorage: tmp,
	})
	require.NoError(t, err, "unable to start openvpn")

	// Wait for client to be connected
	select {
	case <-clientConnected:
	case <-time.After(30 * time.Second):
		t.Fatal("Expected client to connect in less than 30s")
	}
	// Stop VPN client
	vpn.Stop()

	err = vpn.Wait()
	require.NoError(t, err, "Expected no error from vpn.Wait()")
}
