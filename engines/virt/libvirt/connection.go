package libvirt

import (
	"encoding/xml"
	"runtime"
	"sync"

	v "github.com/rgbkrk/libvirt-go"
)

// https://golang.org/pkg/runtime/#SetFinalizer

// Connection holds a connection to hypervisor.
type Connection struct {
	conn      v.VirConnection
	mIsClosed sync.Mutex
	isClosed  bool
}

// NewConnection creates a new Connection to hypervisor.
func NewConnection(uri string) (*Connection, error) {
	conn, err := v.NewVirConnection(uri)
	if err != nil {
		return nil, err
	}
	c := &Connection{
		conn: conn,
	}
	runtime.SetFinalizer(c, func(c *Connection) {
		c.Close()
	})

	return c, nil
}

// Close the Connection (decrements libvirt internal reference count).
func (c *Connection) Close() error {
	c.mIsClosed.Lock()
	defer c.mIsClosed.Unlock()
	if c.isClosed {
		return nil
	}
	c.isClosed = true
	_, err := c.conn.CloseConnection()
	return err
}

// DefineDomain defines a domain and returns a Domain object.
// Remember to Dispose() the domain object if not used.
func (c *Connection) DefineDomain(domain DomainConfig) (*Domain, error) {
	xmlConfig, err := xml.MarshalIndent(domain, "", "  ")
	if err != nil {
		return nil, err
	}

	dom, err := c.conn.DomainDefineXML(string(xmlConfig))
	if err != nil {
		return nil, err
	}
	return newDomain(dom), nil
}
