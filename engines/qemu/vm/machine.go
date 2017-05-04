package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// Machine specifies arguments for various QEMU options.
//
// This only allows certain arguments to be specified.
type Machine struct {
	UUID    string `json:"uuid"`
	Memory  int    `json:"memory,omitempty"`
	Network struct {
		Device string `json:"device"`
		MAC    string `json:"mac"`
	} `json:"network"`
	Keyboard struct {
		Layout string `json:"layout"`
	} `json:"keyboard"`
	Sound *struct {
		Device     string `json:"device"`
		Controller string `json:"controller"`
	} `json:"sound,omitempty"`
	// TODO: Add more options in the future
}

var machineSchema = schematypes.Object{
	Title:       "Machine Definition",
	Description: `Hardware definition for a virtual machine`,
	Properties: schematypes.Properties{
		"uuid": schematypes.String{
			Title:       "System UUID",
			Description: `System UUID for the virtual machine`,
			Pattern:     `^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
		},
		"memory": schematypes.Integer{
			Title:       "Memory",
			Description: `Memory in MiB, defaults to maximum available, if not specified.`,
			Minimum:     0,
			Maximum:     math.MaxInt64,
		},
		"network": schematypes.Object{
			Properties: schematypes.Properties{
				"device": schematypes.StringEnum{
					Title:   "Network Device",
					Options: []string{"rtl8139", "e1000"},
				},
				"mac": schematypes.String{
					Title:       "MAC Address",
					Description: `Local unicast MAC Address`,
					Pattern:     `^[0-9a-f][26ae](:[0-9a-f]{2}){5}$`,
				},
			},
			Required: []string{"device", "mac"},
		},
		"keyboard": schematypes.Object{
			Title: "Keyboard Layout",
			Properties: schematypes.Properties{
				"layout": schematypes.StringEnum{
					Options: []string{
						"ar", "da", "de", "de-ch", "en-gb", "en-us", "es", "et", "fi", "fo",
						"fr", "fr-be", "fr-ca", "fr-ch", "hr", "hu", "is", "it", "ja", "lt",
						"lv", "mk", "nl", "nl-be", "no", "pl", "pt", "pt-br", "ru", "sl",
						"sv", "th", "tr",
					},
				},
			},
			Required: []string{"layout"},
		},
		"sounds": schematypes.AnyOf{
			schematypes.Object{
				Title: "PCI Audio",
				Properties: schematypes.Properties{
					"device": schematypes.StringEnum{
						Title:   "Audio Device",
						Options: []string{"AC97", "ES1370"},
					},
					"controller": schematypes.StringEnum{
						Title:   "Audio Controller",
						Options: []string{"pci"},
					},
				},
				Required: []string{"device", "controller"},
			},
			schematypes.Object{
				Title: "Intel HDA",
				Properties: schematypes.Properties{
					"device": schematypes.StringEnum{
						Title:   "Audio Device",
						Options: []string{"hda-duplex", "hda-micro", "hda-output"},
					},
					"controller": schematypes.StringEnum{
						Title:   "Audio Controller",
						Options: []string{"ich9-intel-hda", "intel-hda"},
					},
				},
				Required: []string{"device", "controller"},
			},
		},
	},
	Required: []string{
		"uuid",
		"network",
		"keyboard",
	},
}

// LoadMachine will load machine definition from file
func LoadMachine(machineFile string) (*Machine, error) {
	// Load the machine configuration
	machineData, err := ioext.BoundedReadFile(machineFile, 1024*1024)
	if err == ioext.ErrFileTooBig {
		return nil, runtime.NewMalformedPayloadError(
			"The file 'machine.json' larger than 1MiB. JSON files must be small.")
	}
	if err != nil {
		return nil, err
	}

	// Parse json
	m := &Machine{}
	err = json.Unmarshal(machineData, m)
	if err != nil {
		return nil, runtime.NewMalformedPayloadError(
			"Invalid JSON in 'machine.json', error: ", err)
	}

	// Validate the definition
	if err := m.Validate(); err != nil {
		return nil, err
	}

	return m, nil
}

// Clone returns a copy of this machine definition
func (m *Machine) Clone() *Machine {
	machine := *m
	return &machine
}

// Validate returns a MalformedPayloadError if the Machine definition isn't
// valid and legal.
func (m *Machine) Validate() error {
	// Render to JSON so we can validate with schematypes
	// (this isn't efficient, but we'll rarely do this so who cares)
	data, err := json.Marshal(m)
	if err != nil {
		panic(fmt.Sprint("json.Marshal should never fail for vm.Machine, error: ", err))
	}
	var v interface{}
	if err = json.Unmarshal(data, &v); err != nil {
		panic(fmt.Sprint("json.Unmarshal should never fail after json.Marshal, error: ", err))
	}

	// Validate against JSON schema
	if err = machineSchema.Validate(v); err != nil {
		return runtime.NewMalformedPayloadError("Invalid machine definition in 'machine.json':", err)
	}

	return nil
}

// validateMAC ensures that MAC address has local bit set, and multicast bit
// unset. This is important as we shouldn't use globally registered MAC
// addresses in our virtual machines.
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

// SetDefaults will validate limitations and set defaults from options
func (m *Machine) SetDefaults(options MachineOptions) error {
	// Set defaults
	if m.Memory == 0 {
		m.Memory = options.MaxMemory
	}

	// Validate limitations
	if m.Memory > options.MaxMemory {
		return runtime.NewMalformedPayloadError(
			"Image memory ", m.Memory, " MiB is larger than allowed machine memory ",
			options.MaxMemory, " MiB",
		)
	}

	return nil
}

// Options will return a set of MachineOptions that allows the current machine
// definition, and otherwise contains sane defaults. This is for utilities only.
func (m Machine) Options() MachineOptions {
	options := MachineOptions{
		MaxMemory: 4 * 1024, // Default 4 GiB memory
	}
	if m.Memory != 0 {
		options.MaxMemory = m.Memory
	}
	return options
}
