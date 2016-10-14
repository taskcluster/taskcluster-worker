package qemurun

import (
	"fmt"
	"io"
	"net"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// socketDisplayProvider is a trivial implementation of
// interactive.DisplayProvider given a unix socket accepting VNC connections.
type socketDisplayProvider struct {
	socket string
}

func (p *socketDisplayProvider) ListDisplays() ([]engines.Display, error) {
	return []engines.Display{
		{
			Name:        "screen",
			Description: "Display offered by QEMU VNC socket.",
		},
	}, nil
}

func (p *socketDisplayProvider) OpenDisplay(name string) (io.ReadWriteCloser, error) {
	if name != "screen" {
		return nil, engines.ErrNoSuchDisplay
	}
	c, err := net.Dial("unix", p.socket)
	if err != nil {
		return nil, fmt.Errorf("Failed open VNC connection, error: %s", err)
	}
	return c, nil
}
