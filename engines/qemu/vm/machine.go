package vm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"github.com/xeipuuv/gojsonschema"
)

// Machine specifies arguments for various QEMU options.
//
// This only allows certain arguments to be specified.
type Machine struct {
	UUID    string `json:"uuid"`
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

// TODO: Find a way that this schema can be included in the documentation...
const machineSchemaString = `{
	"$schema":										"http://json-schema.org/draft-04/schema#",
	"title":											"Machine Definition",
	"description":								"Hardware definition for a virtual machine",
	"type":												"object",
	"properties": {
		"uuid": {
			"type":										"string",
			"pattern":								"^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
			"description":						"System UUID for the virtual machine"
		},
		"network": {
			"type":									 	"object",
			"properties": {
				"device": {
					"type":								"string",
					"description":				"Network device",
					"enum": [
						"rtl8139",
						"e1000"
					]
				},
				"mac": {
					"type":								"string",
					"pattern":						"^[0-9a-f][26ae](:[0-9a-f]{2}){5}$",
					"description":				"Local unicast MAC Address"
				}
			},
			"additionalProperties":		false,
			"required": [
				"device",
				"mac"
			]
		},
		"keyboard": {
			"type":										"object",
			"properties": {
				"layout": {
					"type":								"string",
					"description":				"Keyboard layout",
					"enum": [
						"ar", "da", "de", "de-ch", "en-gb", "en-us", "es", "et", "fi", "fo",
						"fr", "fr-be", "fr-ca", "fr-ch", "hr", "hu", "is", "it", "ja", "lt",
						"lv", "mk", "nl", "nl-be", "no", "pl", "pt", "pt-br", "ru", "sl",
						"sv", "th", "tr"
					]
				}
			},
			"additionalProperties":		false,
			"required": [
				"layout"
			]
		},
		"sound": { "anyOf":
			[{
				"type": 								"object",
				"properties": {
					"device": {
						"type":							"string",
						"description":			"Audio Device",
						"enum": [
							"AC97",
							"ES1370"
						]
					},
					"controller": {
						"type":							"string",
						"description":			"Audio Controller",
						"enum": [
							"pci"
						]
					}
				},
				"additionalProperties":	false,
				"required": [
					"device",
					"controller"
				]
			}, {
				"type": 								"object",
				"properties": {
					"device": {
						"type":							"string",
						"description":			"Audio Device",
						"enum": [
							"hda-duplex",
							"hda-micro",
							"hda-output"
						]
					},
					"controller": {
						"type":							"string",
						"description":			"Audio Controller",
						"enum": [
							"ich9-intel-hda",
							"intel-hda"
						]
					}
				},
				"additionalProperties":	false,
				"required": [
					"device",
					"controller"
				]
			}]
		}
	},
	"additionalProperties":				false,
	"required": [
		"uuid",
		"network",
		"keyboard"
	]
}`

var machineSchema = func() *gojsonschema.Schema {
	schema, err := gojsonschema.NewSchema(
		gojsonschema.NewStringLoader(machineSchemaString),
	)
	if err != nil {
		panic(err)
	}
	return schema
}()

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

	// Render to JSON so we can validate with gojsonschema
	// (this isn't efficient, but we'll rarely do this so who cares)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		panic(fmt.Sprintln(
			"json.Marshal should never fail for vm.Machine, error: ", err,
		))
	}

	// Validate against JSON schema
	result, err := machineSchema.Validate(
		gojsonschema.NewStringLoader(string(data)),
	)
	if err != nil {
		panic(fmt.Sprintln(
			"machineSchema.Validate should always be able to validate, error: ", err,
		))
	}
	if !result.Valid() {
		for _, err := range result.Errors() {
			msg(err.(*gojsonschema.ResultErrorFields).String())
		}
	}

	// Return any errors collected
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
