// +build linux

package network

import (
	"os"
	"syscall"
	"unsafe"
)

const tuntapDevice = "/dev/net/tun"

const (
	ifNameSize = 16
	ifReqSize  = 40
)

type ifReq struct {
	Name  [ifNameSize]byte
	Flags uint16
	pad   [ifReqSize - ifNameSize - 2]byte
}

func createTAPDevice(name string) error {
	f, err := os.OpenFile(tuntapDevice, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	var req ifReq
	copy(req.Name[:ifNameSize-1], name)
	req.Flags = syscall.IFF_TAP | syscall.IFF_NO_PI
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TUNSETIFF), uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		return errno
	}
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TUNSETPERSIST), 1)
	if errno != 0 {
		return errno
	}
	return nil
}

func destroyTAPDevice(name string) error {
	f, err := os.OpenFile(tuntapDevice, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	var req ifReq
	copy(req.Name[:ifNameSize-1], name)
	req.Flags = syscall.IFF_TAP | syscall.IFF_NO_PI
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TUNSETIFF), uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		return errno
	}
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TUNSETPERSIST), 0)
	if errno != 0 {
		return errno
	}
	return nil
}
