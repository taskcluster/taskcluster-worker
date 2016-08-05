package mocknet

import "net"

// MockListener is implementation of net.Listener that allowd for establishment
// of net.Pipe() connections pairs
type MockListener struct {
	addr   MockAddr
	conns  chan net.Conn
	closed chan struct{}
}

// Listen for new mock connections to addr
func Listen(addr string) (*MockListener, error) {
	l := &MockListener{
		addr:   MockAddr{addr: addr},
		conns:  make(chan net.Conn),
		closed: make(chan struct{}),
	}
	mNetworks.Lock()
	defer mNetworks.Unlock()
	if networks[addr] != nil {
		return nil, ErrAddressInUse
	}
	networks[addr] = l
	return l, nil
}

// Accept new connections, see net.Listener.Accept()
func (l *MockListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.closed:
		return nil, ErrListenerClosed
	}
}

// Close closes the listener.
func (l *MockListener) Close() error {
	select {
	case <-l.closed:
		return ErrListenerClosed
	default:
		mNetworks.Lock()
		defer mNetworks.Unlock()
		delete(networks, l.addr.addr)
		close(l.closed)
		return nil
	}
}

// Addr returns the listener's network address.
func (l *MockListener) Addr() net.Addr {
	return &l.addr
}

// Dial creates a new connection to mock network identified by addr
func Dial(addr string) (net.Conn, error) {
	mNetworks.Lock()
	l := networks[addr]
	mNetworks.Unlock()

	if l == nil {
		return nil, ErrConnRefused
	}

	c1, c2 := net.Pipe()
	select {
	case l.conns <- c1:
		return c2, nil
	case <-l.closed:
		return nil, ErrConnRefused
	}
}
