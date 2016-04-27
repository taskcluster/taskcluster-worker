package libvirt

import (
	"runtime"
	"sync"

	v "github.com/rgbkrk/libvirt-go"
)

// Domain references a libvirt domain (virtual machine).
// While this is just a reference it is still important call Dispose(), or
// resources may be leaked.
type Domain struct {
	dom         v.VirDomain
	mIsDisposed sync.Mutex
	isDisposed  bool
}

func newDomain(dom v.VirDomain) *Domain {
	d := &Domain{
		dom: dom,
	}
	runtime.SetFinalizer(d, func(d *Domain) {
		d.Dispose()
	})
	return d
}

// Dispose releases any resources held by this domain.
func (d *Domain) Dispose() error {
	d.mIsDisposed.Lock()
	defer d.mIsDisposed.Unlock()
	if d.isDisposed {
		return nil
	}
	d.isDisposed = true
	return d.dom.Free()
}

// Create starts the virtual machine.
func (d *Domain) Create() error {
	return d.dom.Create()
}

// Destroy hard-kills the virtual machine (pulls the power plug).
func (d *Domain) Destroy() error {
	return d.dom.Destroy()
}

// Undefine deletes the virtual machine.
func (d *Domain) Undefine() error {
	return d.dom.Undefine()
}
