package virt

import (
	"encoding/xml"
	"errors"
)

// PresentTag is a boolean that will be read as true, if the tag is present.
// For this reason it **must** always be used combination with ",omitempty".
type PresentTag bool

// UnmarshalXML implements xml.Unmarshaler for PresentTag
func (b *PresentTag) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*b = true
	return d.Skip()
}

// MarshalXML implements xml.Marshaler for PresentTag
func (b PresentTag) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if !b {
		return errors.New("PresentTag must be used with ,emitempty")
	}
	return e.EncodeElement("", start)
}

// Memory specification for virtual machine.
// https://libvirt.org/formatdomain.html#elementsMemoryAllocation
type Memory struct {
	Size int    `xml:",chardata"`
	Unit string `xml:"unit,attr"`
}

// VCPU specifies the virtual CPU and possible pinning of cpu-set.
// Use Placement="static" with CPUSet="1,2" or Placement="auto" and only
// specify the Maximum.
// https://libvirt.org/formatdomain.html#elementsCPUAllocation
type VCPU struct {
	Maximum   int    `xml:",chardata"`                // 2, number of CPUs
	Placement string `xml:"placement,attr,omitempty"` // "auto" or "static"
	CPUSet    string `xml:"cpuset,attr,omitempty"`    // "1,2" list of CPU to use
}

// OSType specifies the operating system configuration.
// https://libvirt.org/formatdomain.html#elementsOSBIOS
type OSType struct {
	Type         string `xml:",chardata"`    // "hvm", for fully virtualized
	Architecture string `xml:"arch,attr"`    // "x86_64" or "i686"
	Machine      string `xml:"machine,attr"` // "" or any string really
}

// BootMenu specifies if a boot menu should be displayed.
// We always want to disable the bootmenu, to ensure that we must set this.
// https://libvirt.org/formatdomain.html#elementsOSBIOS
type BootMenu struct {
	Enable  string `xml:"enable,attr"`  // "no", always disable boot menu
	Timeout int    `xml:"timeout,attr"` // 0, set timeout for the boot menu
}

// Clock specifies the clock offset.
// https://libvirt.org/formatdomain.html#elementsTime
type Clock struct {
	Offset string `xml:"offset,attr,omitempty"` // "utc", even for Windows
}

// DeviceAddress specifies how a device is attached to the virtual machine.
// These properties are typically optional, and libvirt will generate a defaults
// for values that are omitted (this is probably only for advanced usage).
// https://libvirt.org/formatdomain.html#elementsAddress
type DeviceAddress struct {
	Type     string `xml:"type,attr"`
	Domain   string `xml:"domain,attr,omitempty"`
	Bus      string `xml:"bus,attr,omitempty"`
	Slot     string `xml:"slot,attr,omitempty"`
	Function string `xml:"function,attr,omitempty"`
}

// BootOrder specifies the boot-order of a device.
// This property is usually optional with ",omitempty". Presence indicates that
// the device is bootable, and the optional order property specifies the order.
// https://libvirt.org/formatdomain.html#elementsDisks
type BootOrder struct {
	Order int `xml:"order,attr,omitempty"` // 1, for the first boot device
}

// Disk specifies a disk device. This structure only supports file backed disks.
// https://libvirt.org/formatdomain.html#elementsDisks
type Disk struct {
	Type      string         `xml:"type,attr"`
	Device    string         `xml:"device,attr"`
	ReadOnly  PresentTag     `xml:"readonly,omitempty"`
	Transient PresentTag     `xml:"transient,omitempty"`
	Address   *DeviceAddress `xml:"address,omitempty"`
	Boot      *BootOrder     `xml:"boot,omitempty"`
	Source    struct {
		File string `xml:"file,attr"`
	} `xml:"source"`
	Driver struct {
		Name  string `xml:"name,attr"`
		Type  string `xml:"type,attr"`
		Cache string `xml:"cache,attr,omitempty"`
	} `xml:"driver"`
	Target struct {
		Device string `xml:"dev,attr,omitempty"`
		Bus    string `xml:"bus,attr,omitempty"`
	} `xml:"target"`
}

// Controller specifies a virtual controller. Typically, libvirt can infer these
// and it should be necesary to specify any.
// https://libvirt.org/formatdomain.html#elementsControllers
type Controller struct {
	Type    string         `xml:"type,attr"`  // "pci", "usb", "ide"
	Index   int            `xml:"index,attr"` // Index starting from 0
	Model   string         `xml:"model,attr,omitempty"`
	Ports   int            `xml:"ports,attr,omitempty"`
	Vectors int            `xml:"vectors,attr,omitempty"`
	Address *DeviceAddress `xml:"address,omitempty"`
}

// NetworkInterface specifies a virtual network controller and its attachments.
// This structure allows for attachment of virtual networks and bridge networks.
// https://libvirt.org/formatdomain.html#elementsNICS
type NetworkInterface struct {
	Source struct {
		Network string `xml:"network,attr,omitempty"` // Virtual network name
		Bridge  string `xml:"bridge,attr,omitempty"`  // Bridge network device
	} `xml:"source,omitempty"`
	MAC struct {
		Address string `xml:"address,attr"`
	} `xml:"mac,omitempty"`
	Model   *TypeAttribute `xml:"model,omitempty"` // "..."
	Boot    *BootOrder     `xml:"boot,omitempty"`  // nil
	Address *DeviceAddress `xml:"address,omitempty"`
}

// InputDevice specifies a virtual input device. The "table" allows for absolute
// movement, where as the "mouse" device allows for relative movement.
// https://libvirt.org/formatdomain.html#elementsInput
type InputDevice struct {
	Type    string         `xml:"type,attr"`          // "mouse", "tablet", "keyboard"
	Bus     string         `xml:"bus,attr,omitempty"` // "ps2", "usb", "virtio"
	Address *DeviceAddress `xml:"address,omitempty"`  // nil
}

// GraphicsDevice specifies graphical framebuffer for interaction with the
// virtual machine. This structure allows for specification of the a VNC server.
// https://libvirt.org/formatdomain.html#elementsGraphics
type GraphicsDevice struct {
	Type        string `xml:"type,attr"`                  // "vnc"
	Listen      string `xml:"listen,attr,omitempty"`      // IP to listen on
	Port        int    `xml:"port,attr,omitempty"`        // -1, for automatic
	AutoPort    string `xml:"autoport,attr,omitempty"`    // "yes" or "no"
	SharePolicy string `xml:"sharePolicy,attr,omitempty"` // "force-shared"
}

// VideoDevice specifies a virtual GPU.
// https://libvirt.org/formatdomain.html#elementsVideo
type VideoDevice struct {
	Model struct {
		Type         string `xml:"type,attr"`            // "vga", "cirrus", "vmvga", "xen", "vbox", "qxl"
		VRAM         int    `xml:"vram,attr,omitempty"`  // Video memory in KiB
		RAM          int    `xml:"ram,attr,omitempty"`   // Second memory bar (KiB)
		Heads        int    `xml:"heads,attr,omitempty"` // 1
		Acceleration *struct {
			Accelerate3D string `xml:"accel3d,attr,omitempty"`
			Accelerate2D string `xml:"accel2d,attr,omitempty"`
		} `xml:"acceleration,omitempty"`
	} `xml:"model"`
	Address *DeviceAddress `xml:"address,omitempty"` // nil
}

// TypeAttribute holds the Type attribute for any element which only has this.
type TypeAttribute struct {
	Type string `xml:"type,attr"`
}

// SoundDevice specifies a virtual sound device.
// https://libvirt.org/formatdomain.html#elementsSound
type SoundDevice struct {
	Model   string         `xml:"model,attr"`        // "es1370" "sb16" "ac97" "ich6", "usb"
	Codec   *TypeAttribute `xml:"codec,omitempty"`   // "duplex", "micro" for "ich6"
	Address *DeviceAddress `xml:"address,omitempty"` // nil
}

// ChannelDevice specifies a private communication channel between host and
// virtual machine. This structure specifies a guest forward channel, where
// all traffic to a magic ip:port is forwarded to a unix domain socket on the
// host. This allows for features like EC2 meta-data service.
// https://libvirt.org/formatdomain.html#elementCharChannel
type ChannelDevice struct {
	Type   string `xml:"type,attr"` // "unix"
	Source struct {
		Mode string `xml:"mode,attr,omitempty"` // "bind"
		Path string `xml:"path,attr,omitempty"` // "/tmp/unix-socket"
	} `xml:"source,omitempty"`
	Target struct {
		Type    string `xml:"type,attr,omitempty"`    // "guestfwd"
		Address string `xml:"address,attr,omitempty"` // Magic IP to forward from
		Port    int    `xml:"port,attr,omitempty"`    // Port on magic IP
	} `xml:"target,omitempty"`
}

// Domain specifies a libvirt domain, also known as a virtual machine.
// https://libvirt.org/formatdomain.html
type Domain struct {
	// TODO: See hyperv features add these!
	// See: https://libvirt.org/formatdomain.html#elementsFeatures
	XMLName           xml.Name           `xml:"domain"`
	Type              string             `xml:"type,attr"` // "kvm"
	Name              string             `xml:"name"`      // Identifier
	UUID              string             `xml:"uuid"`      // UUID
	Memory            Memory             `xml:"memory"`
	CurrentMemory     Memory             `xml:"currentMemory"`
	VCPU              VCPU               `xml:"vcpu"`
	OSType            OSType             `xml:"os>type"`
	BootMenu          BootMenu           `xml:"os>bootmenu"`
	PAE               PresentTag         `xml:"features>pae,omitempty"`
	ACPI              PresentTag         `xml:"features>acpi,omitempty"`
	APIC              PresentTag         `xml:"features>apic,omitempty"`
	Clock             Clock              `xml:"clock"`
	OnPowerOff        string             `xml:"on_poweroff,omitempty"`    // "destroy"
	OnReboot          string             `xml:"on_reboot,omitempty"`      // "destroy"
	OnCrash           string             `xml:"on_crash,omitempty"`       // "destroy"
	OnLockFailure     string             `xml:"on_lockfailure,omitempty"` // "destroy"
	Emulator          string             `xml:"devices>emulator"`         // "/usr/bin/kvm"
	Disks             []Disk             `xml:"devices>disk"`
	Controllers       []Controller       `xml:"devices>controller"`
	NetworkInterfaces []NetworkInterface `xml:"devices>interface"`
	InputDevices      []InputDevice      `xml:"devices>input"`
	GraphicsDevices   []GraphicsDevice   `xml:"devices>graphics"`
	VideoDevices      []VideoDevice      `xml:"devices>video"`
	SoundDevices      []SoundDevice      `xml:"devices>sound"`
	ChannelDevices    []ChannelDevice    `xml:"devices>channel"`
}
