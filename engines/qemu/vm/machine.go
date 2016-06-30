package vm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// Machine specifies arguments for various QEMU options.
//
// This only allows certain arguments to be specified.
type Machine struct {
	Network struct {
		Device string `json:"device"` // "rtl8139"
		MAC    string `json:"mac"`    // Set local bit, ensure unicast address
	} `json:"network"`
	// TODO: Add more options in the future
	// TODO: Add UUID

	// TODO: Specify this with a JSON schema instead
}

// LoadMachine will load machine definition from file
func LoadMachine(machineFile string) (*Machine, error) {
	// Load the machine configuration
	machineData, err := ioext.BoundedReadFile(machineFile, 1024*1024)
	if err == ioext.ErrFileTooBig {
		return nil, engines.NewMalformedPayloadError(
			"The file 'machine.json' larger than 1MiB. JSON files must be small.")
	}
	if err != nil {
		return nil, err
	}

	// Parse json
	m := &Machine{}
	err = json.Unmarshal(machineData, m)
	if err != nil {
		return nil, engines.NewMalformedPayloadError(
			"Invalid JSON in 'machine.json', error: ", err)
	}

	// Validate the defintion
	if err := m.Validate(); err != nil {
		return nil, err
	}

	return m, nil
}

// Clone returns a copy of this machine definition
func (m *Machine) Clone() *Machine {
	var machine Machine
	machine = *m
	return &machine
}

// Validate returns a MalformedPayloadError if the Machine definition isn't
// valid and legal.
func (m *Machine) Validate() error {
	hasError := false
	errs := "Invalid machine defintion in 'machine.json'"
	msg := func(a ...interface{}) {
		errs += "\n" + fmt.Sprint(a...)
		hasError = true
	}

	// Validate network device
	if m.Network.Device != "rtl8139" && m.Network.Device != "e1000" {
		msg("network.device must be 'rtl8139', but '", m.Network.Device, "' was given")
	}

	// Validate the MAC address
	err := validateMAC(m.Network.MAC)
	if err != nil {
		msg(err.Error())
	}

	if hasError {
		return engines.NewMalformedPayloadError(errs)
	}
	return nil
}

// validateMAC ensures that MAC address has local bit set, and multicast bit
// unset. This is important as we shouldn't use globally registered MAC
// addreses in our virtual machines.
func validateMAC(mac string) error {
	m := make([]byte, 6)
	n, err := fmt.Sscanf(
		mac, "%02x:%02x:%02x:%02x:%02x:%02x",
		&m[0], &m[1], &m[2], &m[3], &m[4], &m[5],
	)
	if n != 6 && err != nil {
		return errors.New("MAC address must be on the form: x:x:x:x:x:x")
	} else if m[0]&2 == 0 {
		return errors.New("MAC address must have the local bit set")
	} else if m[0]&1 == 1 {
		return errors.New("MAC address must have the multicast bit unset")
	}
	return nil
}
