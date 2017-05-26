package vm

import (
	"encoding/json"
	"fmt"
	"reflect"
	rt "runtime"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// version number of the machine.json format
const machineFormatVersion = 1

// Machine specifies arguments for various QEMU options.
//
// Machine objects can be combined using the WithDefaults() method which creates
// a new Machine from two Machine objects, merging the machine definitions.
// The empty Machine value specifies nothing and will be fully overwritten with
// defaults if used as target of WithDefaults(). Effectively, the empty Machine
// definition will assume whatever default values are given.
type Machine struct {
	options struct {
		Version        int      `json:"version"` // see machineFormatVersion
		UUID           string   `json:"uuid"`
		Chipset        string   `json:"chipset"`
		CPU            string   `json:"cpu"`
		Flags          []string `json:"flags"`
		Threads        int      `json:"threads"`
		Cores          int      `json:"cores"`
		Sockets        int      `json:"sockets"`
		Memory         int      `json:"memory"`
		USB            string   `json:"usb"`
		Network        string   `json:"network"`
		MAC            string   `json:"mac"`
		Storage        string   `json:"storage"`
		Graphics       string   `json:"graphics"`
		VGAMemory      int      `json:"vgaMemory"`
		GraphicsRAM    int      `json:"graphicsRam"`
		GraphicsVRAM   int      `json:"graphicsVRam"`
		Sound          string   `json:"sound"`
		Keyboard       string   `json:"keyboard"`
		KeyboardLayout string   `json:"keyboardLayout"`
		Mouse          string   `json:"mouse"`
		Tablet         string   `json:"tablet"`
	}
}

var defaultMachine = (func() Machine {
	var m Machine
	err := json.Unmarshal([]byte(`{
		"version":         1,
		"uuid":            "52bab607-10f1-4049-a0f8-ee4725cb715b",
		"chipset":         "pc-i440fx-2.8",
		"cpu":             "host",
		"flags":           [],
		"usb":             "nec-usb-xhci",
		"network":         "e1000",
		"mac":             "aa:54:1a:30:5c:de",
		"storage":         "virtio-blk-pci",
		"graphics":        "qxl-vga",
		"vgaMemory":       16,
		"graphicsRam":     64,
		"graphicsVRam":    32,
		"sound":           "none",
		"keyboard":        "usb-kbd",
		"keyboardLayout":  "en-us",
		"mouse":           "usb-mouse",
		"tablet":          "usb-tablet"
	}`), &m.options)
	if err != nil {
		panic("failed to parse static JSON config")
	}
	return m
})()

// NewMachine returns a new machine from definition matching MachineSchema
func NewMachine(definition interface{}) Machine {
	var m Machine
	schematypes.MustValidateAndMap(MachineSchema, definition, &m.options)
	return m
}

// MarshalJSON implements json.Marshaler
func (m Machine) MarshalJSON() ([]byte, error) {
	// We need a custom implementation because we distinguish between
	// flags nil and empty array.
	result := map[string]interface{}{}
	v := reflect.ValueOf(&m.options).Elem()
	for i := 0; i < v.NumField(); i++ {
		switch v.Field(i).Kind() {
		case reflect.Chan, reflect.Func, reflect.Slice, reflect.Map, reflect.Ptr, reflect.Interface:
			if v.Field(i).IsNil() {
				continue // Skip nil values, but not zero values
			}
		default:
			if v.Field(i).Interface() == reflect.Zero(v.Field(i).Type()).Interface() {
				continue // Skip zero values
			}
		}
		result[v.Type().Field(i).Tag.Get("json")] = v.Field(i).Interface()
	}
	// Always set version
	result["version"] = machineFormatVersion
	return json.Marshal(result)
}

// WithDefaults creates a new Machine with empty-values in c
// defaulting to defaults.
func (m Machine) WithDefaults(defaults Machine) Machine {
	options := m.options
	v := reflect.ValueOf(&options).Elem()
	d := reflect.ValueOf(&defaults.options).Elem()
	for i := 0; i < v.NumField(); i++ {
		switch v.Field(i).Kind() {
		case reflect.Chan, reflect.Func, reflect.Slice, reflect.Map, reflect.Ptr, reflect.Interface:
			if v.Field(i).IsNil() {
				v.Field(i).Set(d.Field(i))
			}
		default:
			if v.Field(i).Interface() == reflect.Zero(v.Field(i).Type()).Interface() {
				v.Field(i).Set(d.Field(i))
			}
		}
	}
	return Machine{options: options}
}

// ApplyLimits returns an Machine with defaults extracted from the limits, or
// a MalformedPayloadError if limits were violated.
func (m Machine) ApplyLimits(limits MachineLimits) (Machine, error) {
	o := m.options

	// Set defaults for memory
	if o.Memory == 0 {
		o.Memory = limits.MaxMemory
	}

	// Always default to at-least one thread, one core and one socket
	if o.Threads == 0 {
		o.Threads = 1
	}
	if o.Cores == 0 {
		o.Cores = 1
	}
	if o.Sockets == 0 {
		o.Sockets = 1
	}

	// If threads wasn't set, and sufficient CPUs is available, we set it to
	// default threads. If neither threads or cores is specified, we
	// want DefaultThreads (if possible) and as many cores as allowed by MaxCPUs
	if m.options.Threads == 0 && limits.DefaultThreads*o.Cores*o.Sockets <= limits.MaxCPUs {
		o.Threads = limits.DefaultThreads
	}
	// If cores was not defined and we can increase it by one, we do so
	for m.options.Cores == 0 && o.Threads*(o.Cores+1)*o.Sockets <= limits.MaxCPUs {
		o.Cores++
	}
	// same for threads and sockets, notice that we prefer to increase sockets
	for m.options.Threads == 0 && (o.Threads+1)*o.Cores*o.Sockets <= limits.MaxCPUs {
		o.Threads++
	}
	for m.options.Sockets == 0 && o.Threads*o.Cores*(o.Sockets+1) <= limits.MaxCPUs {
		o.Sockets++
	}

	// Validate limitations for memory
	if o.Memory > limits.MaxMemory {
		return Machine{o}, runtime.NewMalformedPayloadError(
			"Machine memory ", o.Memory, " MiB is larger than allowed machine memory ",
			limits.MaxMemory, " MiB",
		)
	}

	// Validate limitations for cpu cores
	if o.Threads*o.Cores*o.Sockets > limits.MaxCPUs {
		return Machine{o}, runtime.NewMalformedPayloadError(fmt.Sprintf(
			"Machine specifies threads: %d, cores: %d, sockets: %d, in total: %d "+
				"which is larger than %d total number of cores allowed",
			o.Threads, o.Cores, o.Sockets,
			o.Threads*o.Cores*o.Sockets,
			limits.MaxCPUs,
		))
	}
	return Machine{o}, nil
}

// DeriveLimits constructs sane MachineLimits that permits the machine.
func (m Machine) DeriveLimits() MachineLimits {
	// Default 1 for threads, cores and sockets
	threads := m.options.Threads
	if m.options.Threads == 0 {
		threads = 1
	}
	cores := m.options.Cores
	if m.options.Cores == 0 {
		cores = 1
	}
	sockets := m.options.Sockets
	if m.options.Sockets == 0 {
		sockets = 1
	}

	// Derive max CPUs
	maxCPUs := threads * cores * sockets

	// Allow more CPU is host has more
	if maxCPUs < rt.NumCPU() {
		maxCPUs = rt.NumCPU()
	}

	return MachineLimits{
		MaxMemory:      m.options.Memory,
		MaxCPUs:        maxCPUs,
		DefaultThreads: 1,
	}
}

// MachineSchema must be satisfied by config passed to NewMachine
var MachineSchema schematypes.Schema = schematypes.Object{
	Title:       "Machine Configuration",
	Description: `Hardware definition for a virtual machine`,
	Properties: schematypes.Properties{
		"version": schematypes.IntegerEnum{
			Title:   "Format Version",
			Options: []int{machineFormatVersion},
		},
		"uuid": schematypes.String{
			Title:       "System UUID",
			Description: `System UUID for the virtual machine`,
			Pattern:     `^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
		},
		"chipset": schematypes.StringEnum{
			Title:   "Chipset",
			Options: []string{"pc-i440fx-2.8"},
		},
		"cpu": schematypes.StringEnum{
			Title: "CPU",
			Description: util.Markdown(`
				CPU to be exposed to the virtual machine.

				The number of virtual CPUs inside the virtual machine will be
				'threads * cores * sockets' as configured below.
			`),
			Options: []string{"host"},
		},
		"flags": schematypes.Array{
			Items: schematypes.StringEnum{},
		},
		"threads": schematypes.Integer{
			Description: "Threads per CPU core, leave undefined to get maximum available",
			Minimum:     1,
			Maximum:     255,
		},
		"cores": schematypes.Integer{
			Description: "CPU cores per socket, leave undefined to get maximum available",
			Minimum:     1,
			Maximum:     255,
		},
		"sockets": schematypes.Integer{
			Description: "CPU sockets in machine, leave undefined to get maximum available",
			Minimum:     1,
			Maximum:     255,
		},
		"memory": schematypes.Integer{
			Title:       "Memory",
			Description: `Memory in MiB, defaults to maximum available, if not specified.`,
			Minimum:     1,
			Maximum:     1024 * 1024, // 1 TiB
		},
		"usb": schematypes.StringEnum{
			Title:   "Host Controller Interface",
			Options: []string{"piix3-usb-uhci", "piix4-usb-uhci", "nec-usb-xhci"},
		},
		"network": schematypes.StringEnum{
			Title:   "Network Interface Controller",
			Options: []string{"rtl8139", "e1000"},
		},
		"mac": schematypes.String{
			Title:       "MAC Address",
			Description: `Local unicast MAC Address`,
			Pattern:     `^[0-9a-f][26ae](:[0-9a-f]{2}){5}$`,
		},
		"storage": schematypes.StringEnum{
			Title:       "Storage Device",
			Description: `Block device to use for attaching storage`,
			Options:     []string{"virtio-blk-pci"},
		},
		"graphics": schematypes.StringEnum{
			Options: []string{"VGA", "vmware-svga", "qxl-vga", "virtio-vga"},
		},
		"vgaMemory": schematypes.Integer{
			Title: "VGA Memory",
			Description: util.Markdown(`
				VGA memory is the 'vgamem' option given to QEMU in MB.

				Typically, this should be 'width * height * 4', it defaults to
				16MB which will do for must use-cases.
			`),
			Minimum: 1,
			Maximum: 1024 * 1024, // 1 TiB
		},
		"graphicsRam": schematypes.Integer{
			Title: "Graphics RAM",
			Description: util.Markdown(`
				Graphics RAM is the 'ram' option passed to the 'qxl-vga' device
				in MB. This option will be ignored if not using 'qxl-vga'.

				Typically, this should be '4 * vgaMemory', it defaults to
				128MB which will do for must use-cases.
			`),
			Minimum: 1,
			Maximum: 1024 * 1024, // 1 TiB
		},
		"graphicsVRam": schematypes.Integer{
			Title: "Graphics Virtual RAM",
			Description: util.Markdown(`
				Graphics Virtual RAM is the 'vram' option passed to the 'qxl-vga'
				device in MB. This option will be ignored if not using 'qxl-vga'.

				Typically, this should be '2 * vgaMemory', it defaults to
				32MB which will do for must use-cases.
			`),
			Minimum: 1,
			Maximum: 1024 * 1024, // 1 TiB
		},
		"sound": schematypes.StringEnum{
			Options: []string{
				"none",
				"AC97", "ES1370",
				"hda-duplex/intel-hda", "hda-micro/intel-hda", "hda-output/intel-hda",
				"hda-duplex/ich9-intel-hda", "hda-micro/ich9-intel-hda", "hda-output/ich9-intel-hda",
			},
		},
		"keyboard": schematypes.StringEnum{
			Options: []string{"usb-kbd", "PS/2"},
		},
		"keyboardLayout": schematypes.StringEnum{
			Options: []string{
				"ar", "da", "de", "de-ch", "en-gb", "en-us", "es", "et", "fi", "fo",
				"fr", "fr-be", "fr-ca", "fr-ch", "hr", "hu", "is", "it", "ja", "lt",
				"lv", "mk", "nl", "nl-be", "no", "pl", "pt", "pt-br", "ru", "sl",
				"sv", "th", "tr",
			},
		},
		"mouse": schematypes.StringEnum{
			Options: []string{"usb-mouse", "PS/2"},
		},
		"tablet": schematypes.StringEnum{
			Options: []string{"usb-tablet", "none"},
		},
	},
	Required: []string{"version"},
}
