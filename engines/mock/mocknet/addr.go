package mocknet

// MockAddr is an net.Addr implementation for MockListener
type MockAddr struct {
	addr string
}

// Network returns a network type identifier, like net.Addr.Network()
func (a *MockAddr) Network() string {
	return "mock"
}

// String returns address on string form, like net.Addr.String()
func (a *MockAddr) String() string {
	return "mock:" + a.addr
}
