package vm

import "encoding/json"

// A migration upgrades machine definition from one version to the next
// these will be chained to ensure machine definitions are fully migrated.
//
// If migration fails, method should return nil.
var migrations = []func(map[string]interface{}) map[string]interface{}{
	// Note: Array index must match with the version the function migrates from.
	//       All migrations must migrate to the next version, this way we only
	//       have to write one migration when we change the format.
	migrate0to1,

	// As a final step after migrations we validate against current schema and
	// return nil, if it's not valid.
	func(def map[string]interface{}) map[string]interface{} {
		if MachineSchema.Validate(def) != nil {
			return nil
		}
		return def
	},
}

// MigrateMachineDefinition takes a machine definition and migrates it to the
// latest format version, and returns nil, if format is not supported.
func MigrateMachineDefinition(definition interface{}) interface{} {
	def, ok := definition.(map[string]interface{})
	if !ok {
		return nil
	}
	version := 0 // default version from before we specified version numbers
	if v, ok := def["version"]; ok {
		if ver, ok := v.(float64); ok {
			version = int(ver)
		} else {
			return nil
		}
	}
	for i := version; i < len(migrations) && def != nil; i++ {
		// Normalizing JSON
		raw, _ := json.Marshal(def)
		def = nil
		if len(raw) == 0 || json.Unmarshal(raw, &def) != nil {
			return nil
		}
		// Execution migration step
		def = migrations[i](def)
	}
	return def
}

// Migrate version 0 -> version 1
func migrate0to1(def map[string]interface{}) map[string]interface{} {
	// Note: version 0 did not carry a version number.

	// machineV0 is an old machine definition no longer in use
	type machineV0 struct {
		UUID   string `json:"uuid"`
		Memory int    `json:"memory,omitempty"`
		CPU    struct {
			Threads int `json:"threads,omitempty"`
			Cores   int `json:"cores,omitempty"`
			Sockets int `json:"sockets,omitempty"`
		} `json:"cpu"`
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
	}

	// Parse JSON
	raw, _ := json.Marshal(def)
	var m machineV0
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		return nil
	}

	// Construct sound definition
	sound := "none"
	if m.Sound != nil {
		sound = m.Sound.Device
		if m.Sound.Controller != "pci" {
			sound += "/" + m.Sound.Controller
		}
	}

	// Create result
	result := map[string]interface{}{
		"version":        1,
		"uuid":           m.UUID,
		"chipset":        "pc-i440fx-2.8",
		"cpu":            "host",
		"threads":        1, // always default to 1
		"flags":          []interface{}{},
		"usb":            "nec-usb-xhci",
		"network":        "e1000",
		"mac":            m.Network.MAC,
		"storage":        "virtio-blk-pci",
		"graphics":       "vmware-svga",
		"sound":          sound,
		"keyboard":       "usb-kbd",
		"keyboardLayout": m.Keyboard.Layout,
		"mouse":          "usb-mouse",
		"tablet":         "none",
	}

	// Add threads, cores, sockets, and memory if specified
	if m.CPU.Threads != 0 {
		result["threads"] = m.CPU.Threads
	}
	if m.CPU.Cores != 0 {
		result["cores"] = m.CPU.Cores
	}
	if m.CPU.Sockets != 0 {
		result["sockets"] = m.CPU.Sockets
	}
	if m.Memory != 0 {
		result["memory"] = m.Memory
	}

	return result
}
