// +build monitor

package runtime

import (
	"fmt"
	"os"
	"testing"

	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/auth"
)

func TestLoggingMonitor(t *testing.T) {
	m := NewLoggingMonitor("debug", nil)
	m.Debug("hello world")
	m.Measure("my-measure", 56)
	m.Count("my-counter", 1)
	m.WithPrefix("my-prefix").Count("counter-2", 1)
	m.WithPrefix("my-prefix").Info("info message")
	m.WithTag("myTag", "myValue").Warn("some warning")
	m.WithTag("myTag", "myValue").ReportWarning(fmt.Errorf("error message"), "this is a warning")
}

func TestMonitor(t *testing.T) {
	clientID := os.Getenv("TASKCLUSTER_CLIENT_ID")
	accessToken := os.Getenv("TASKCLUSTER_ACCESS_TOKEN")
	if clientID == "" || accessToken == "" {
		t.Skip("TASKCLUSTER_CLIENT_ID and TASKCLUSTER_ACCESS_TOKEN not defined")
	}
	a := auth.New(&tcclient.Credentials{
		ClientID:    clientID,
		AccessToken: accessToken,
	})

	m := NewMonitor("test-dummy-worker", a, "debug", nil)
	m.Debug("hello world")
	m.Measure("my-measure", 56)
	m.Count("my-counter", 1)
	m.WithPrefix("my-prefix").Count("counter-2", 1)
	m.WithPrefix("my-prefix").Info("info message")
	m.WithTag("myTag", "myValue").Warn("some warning")
	m.WithTag("myTag", "myValue").ReportWarning(fmt.Errorf("error message"), "this is a warning")
}
