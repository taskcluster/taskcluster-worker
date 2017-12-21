package vm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrateFromInvalid(t *testing.T) {
	var def interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"uuid": "52bab607-10f1-4049-a0f8-ee4725cb715b",
		"network": "e1000",
		"keyboard": {
			"layout": "en-us"
		}
	}`), &def))
	def = MigrateMachineDefinition(def)
	assert.Nil(t, def, "Expected no machine definition")
}

func TestMigrateFromV0(t *testing.T) {
	var def interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"uuid": "52bab607-10f1-4049-a0f8-ee4725cb715b",
		"network": {
			"device": "e1000",
			"mac": "aa:54:1a:30:5c:de"
		},
		"keyboard": {
			"layout": "en-us"
		}
	}`), &def))
	def = MigrateMachineDefinition(def)
	assert.NotNil(t, def, "Expected some machine definition")
}

func TestMigrateFromV1(t *testing.T) {
	var def interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
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
		"sound":           "none",
		"keyboard":        "usb-kbd",
		"keyboardLayout":  "en-us",
		"mouse":           "usb-mouse",
		"tablet":          "usb-tablet"
	}`), &def))
	def = MigrateMachineDefinition(def)
	assert.NotNil(t, def, "Expected some machine definition")
}
