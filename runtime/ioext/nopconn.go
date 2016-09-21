package ioext

import (
	"io"
	"net"
	"time"
)

// nopConn wraps io.ReadWriteCloser as net.Conn
type nopConn struct {
	io.ReadWriteCloser
}

func (c nopConn) LocalAddr() net.Addr {
	return nil
}

func (c nopConn) RemoteAddr() net.Addr {
	return nil
}

func (c nopConn) SetDeadline(time.Time) error {
	return nil
}

func (c nopConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c nopConn) SetWriteDeadline(time.Time) error {
	return nil
}

// NopConn wraps conn so that it provides a trivial implementation of net.Conn.
// This is only useful for testing, deadlines are ignored and address methods
// will return nil.
func NopConn(conn io.ReadWriteCloser) net.Conn {
	return nopConn{conn}
}
