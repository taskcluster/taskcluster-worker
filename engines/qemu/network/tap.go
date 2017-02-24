// +build linux

package network

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
