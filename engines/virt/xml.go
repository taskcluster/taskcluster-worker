package virt

import (
	"crypto/rand"
	"encoding/xml"
	"errors"
	"fmt"
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
	Type         string `xml:",chardata"`              // "hvm", for fully virtualized
	Architecture string `xml:"arch,attr"`              // "x86_64" or "i686"
	Machine      string `xml:"machine,attr,omitempty"` // "" or any string really
}

// BootMenu specifies if a boot menu should be displayed.
// We always want to disable the bootmenu, to ensure that we must set this.
// https://libvirt.org/formatdomain.html#elementsOSBIOS
type BootMenu struct {
	Enable  string `xml:"enable,attr"`  // "no", always disable boot menu
	Timeout int    `xml:"timeout,attr"` // 0, set timeout for the boot menu
}

// DefaultBootMenu specifies a bootmenu that is disabled and zero timeout.
// We don't even want wait for a bootmenu timeout in automation.
var DefaultBootMenu = BootMenu{Enable: "no", Timeout: 0}

// Clock specifies the clock offset.
// https://libvirt.org/formatdomain.html#elementsTime
type Clock struct {
	Offset string `xml:"offset,attr,omitempty"` // "utc", even for Windows
}

// DefaultClock specifies Clock with offset="utc", the only sane setting.
var DefaultClock = Clock{Offset: "utc"}

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

// DefaultBootOrder specifies boot-order 1, meaning the device is the first
// to be booted.
var DefaultBootOrder = &BootOrder{Order: 1}

// DiskSource specifies the location of a source file (for disks).
type DiskSource struct {
	Path string `xml:"file,attr"`
}

// DiskDriver specifies a disk driver (for disks).
type DiskDriver struct {
	Name  string `xml:"name,attr"` // "qemu"
	Type  string `xml:"type,attr"` // "raw"
	Cache string `xml:"cache,attr,omitempty"`
}

// DefaultRawDiskDriver specifies a disk driver for raw images.
var DefaultRawDiskDriver = DiskDriver{Name: "qemu", Type: "raw"}

// DiskTarget specifies a disk target (for disks).
type DiskTarget struct {
	Device string `xml:"dev,attr,omitempty"` // "vda"
	Bus    string `xml:"bus,attr,omitempty"` // "virtio"
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
	Source    DiskSource     `xml:"source"`
	Driver    DiskDriver     `xml:"driver"`
	Target    DiskTarget     `xml:"target"`
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

// DefaultControllers specifies a list of PCI, IDE and USB contollers.
var DefaultControllers = []Controller{
	{Type: "pci", Index: 0, Model: "pci-root"},
	{Type: "ide", Index: 0},
	{Type: "usb", Index: 0},
}

// NetworkSource specifies a network source. Either a virtual network name,
// or a bridge network device.
type NetworkSource struct {
	Network string `xml:"network,attr,omitempty"` // Virtual network name
	Bridge  string `xml:"bridge,attr,omitempty"`  // Bridge network device
}

// MAC specifies a MAC-address for a network interface.
type MAC struct {
	Address string `xml:"address,attr"`
}

// NewMAC generates a new random MAC with the local bit set.
func NewMAC() *MAC {
	// Credits: http://stackoverflow.com/a/21027407/68333
	// Get some random data
	m := make([]byte, 6)
	_, err := rand.Read(m)
	if err != nil {
		panic(err)
	}
	m[0] = (m[0] | 2) & 0xfe // Set local bit, ensure unicast address
	return &MAC{
		Address: fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", m[0], m[1], m[2], m[3], m[4], m[5]),
	}
}

// Parameter specifies a network filter parameter.
type Parameter struct {
	Name  string `xml:"name,attr,omitempty"`
	Value string `xml:"value,attr,omitempty"`
}

// FilterReference references a network filter with given parameters.
type FilterReference struct {
	Name       string      `xml:"filter,attr,omitempty"`
	Parameters []Parameter `xml:"parameter,omitempty"`
}

// NetworkInterface specifies a virtual network controller and its attachments.
// This structure allows for attachment of virtual networks and bridge networks.
// https://libvirt.org/formatdomain.html#elementsNICS
type NetworkInterface struct {
	Type    string           `xml:"type,attr,omitempty"`
	Source  NetworkSource    `xml:"source,omitempty"`
	MAC     *MAC             `xml:"mac,omitempty"`
	Model   *TypeAttribute   `xml:"model,omitempty"` // "..."
	Boot    *BootOrder       `xml:"boot,omitempty"`  // nil
	Address *DeviceAddress   `xml:"address,omitempty"`
	Filter  *FilterReference `xml:"filterref,omitempty"`
}

// InputDevice specifies a virtual input device. The "table" allows for absolute
// movement, where as the "mouse" device allows for relative movement.
// https://libvirt.org/formatdomain.html#elementsInput
type InputDevice struct {
	Type    string         `xml:"type,attr"`          // "mouse", "tablet", "keyboard"
	Bus     string         `xml:"bus,attr,omitempty"` // "ps2", "usb", "virtio"
	Address *DeviceAddress `xml:"address,omitempty"`  // nil
}

// DefaultMouseDevice specifies a USB mouse.
var DefaultMouseDevice = InputDevice{Type: "mouse", Bus: "usb"}

// DefaultKeyboardDevice specifies a USB keyboard.
var DefaultKeyboardDevice = InputDevice{Type: "keyboard", Bus: "usb"}

// GraphicsDevice specifies graphical framebuffer for interaction with the
// virtual machine. This structure allows for specification of the a VNC server.
// https://libvirt.org/formatdomain.html#elementsGraphics
type GraphicsDevice struct {
	Type        string `xml:"type,attr"`                  // "vnc"
	Listen      string `xml:"listen,attr,omitempty"`      // IP to listen on
	Port        int    `xml:"port,attr,omitempty"`        // -1, for automatic or large than 5900 (some obscure bug)
	AutoPort    string `xml:"autoport,attr,omitempty"`    // "yes" or "no"
	SharePolicy string `xml:"sharePolicy,attr,omitempty"` // "force-shared"
}

// VideoAcceleration specifies whether video device has 2D and or 3D acceleration.
type VideoAcceleration struct {
	Accelerate3D string `xml:"accel3d,attr,omitempty"`
	Accelerate2D string `xml:"accel2d,attr,omitempty"`
}

// VideoModel Specifies a video device model.
type VideoModel struct {
	Type         string             `xml:"type,attr"`            // "vga", "cirrus", "vmvga", "xen", "vbox", "qxl"
	VRAM         int                `xml:"vram,attr,omitempty"`  // Video memory in KiB
	RAM          int                `xml:"ram,attr,omitempty"`   // Second memory bar (KiB)
	Heads        int                `xml:"heads,attr,omitempty"` // 1
	Acceleration *VideoAcceleration `xml:"acceleration,omitempty"`
}

// VideoDevice specifies a virtual GPU.
// https://libvirt.org/formatdomain.html#elementsVideo
type VideoDevice struct {
	Model   VideoModel     `xml:"model"`
	Address *DeviceAddress `xml:"address,omitempty"` // nil
}

// DefaultVideoDevice specifies a reasonable default video device.
var DefaultVideoDevice = VideoDevice{
	Model: VideoModel{
		Type:  "cirrus",
		VRAM:  9216, // TODO: Determine if this is a good default.
		Heads: 1,
	},
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

// DefaultSoundDevice specifies a default sound device.
var DefaultSoundDevice = SoundDevice{Model: "ich6"}

// ChannelSource specifies the source for a channel device. In this form
// it'll always be a unix domain socket.
type ChannelSource struct {
	Mode    string `xml:"mode,attr,omitempty"` // "bind"
	Host    string `xml:"host,attr,omitempty"`
	Service int    `xml:"service,attr,omitempty"`
	Path    string `xml:"path,attr,omitempty"` // "/tmp/unix-socket"
}

// ChannelTarget specifies the target for a channel device. In this form it'll
// always be a magic IP address and port.
type ChannelTarget struct {
	Type    string `xml:"type,attr,omitempty"`    // "guestfwd"
	Address string `xml:"address,attr,omitempty"` // Magic IP to forward from
	Port    int    `xml:"port,attr,omitempty"`    // Port on magic IP
}

// ChannelDevice specifies a private communication channel between host and
// virtual machine. This structure specifies a guest forward channel, where
// all traffic to a magic ip:port is forwarded to a unix domain socket on the
// host. This allows for features like EC2 meta-data service.
// https://libvirt.org/formatdomain.html#elementCharChannel
type ChannelDevice struct {
	Type     string         `xml:"type,attr"` // "unix"
	Source   ChannelSource  `xml:"source,omitempty"`
	Target   ChannelTarget  `xml:"target,omitempty"`
	Protocol *TypeAttribute `xml:"protocol,omitempty"`
}

// Domain specifies a libvirt domain, also known as a virtual machine.
// https://libvirt.org/formatdomain.html
type Domain struct {
	// TODO: See hyperv features add these!
	// See: https://libvirt.org/formatdomain.html#elementsFeatures
	XMLName           xml.Name           `xml:"domain"`
	Type              string             `xml:"type,attr"`      // "kvm"
	Name              string             `xml:"name"`           // Identifier
	UUID              string             `xml:"uuid,omitempty"` // UUID
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
	Disks             []Disk             `xml:"devices>disk,omitempty"`
	Controllers       []Controller       `xml:"devices>controller,omitempty"`
	NetworkInterfaces []NetworkInterface `xml:"devices>interface,omitempty"`
	InputDevices      []InputDevice      `xml:"devices>input,omitempty"`
	GraphicsDevices   []GraphicsDevice   `xml:"devices>graphics,omitempty"`
	VideoDevices      []VideoDevice      `xml:"devices>video,omitempty"`
	SoundDevices      []SoundDevice      `xml:"devices>sound,omitempty"`
	ChannelDevices    []ChannelDevice    `xml:"devices>channel,omitempty"`
}

// IPAddressRange specifies an IP-address range.
type IPAddressRange struct {
	Start string `xml:"start,attr,omitempty"`
	End   string `xml:"end,attr,omitempty"`
}

// PortRange specifies a TCP Port range.
type PortRange struct {
	Start int `xml:"start,attr,omitempty"`
	End   int `xml:"end,attr,omitempty"`
}

// NetworkForwarder specifies a network forwarding strategy.
// This struct is aim at defining NAT forwarding.
type NetworkForwarder struct {
	Mode         string          `xml:"mode,attr,omitempty"`   // "nat"
	NATAddresses *IPAddressRange `xml:"nat>address,omitempty"` // nil
	NATPorts     *PortRange      `xml:"nat>port,omitempty"`    // 1024 - 65535
}

// NetworkBridge specifies a network bridge attachment.
type NetworkBridge struct {
	Name            string `xml:"name,attr,omitempty"` // "virbrX", where X is a number
	STP             string `xml:"stp,attr,omitempty"`  // "on"
	Delay           int    `xml:"delay,attr"`          // 0, always zero!
	MACTableManager string `xml:",attr,omitempty"`     // "libvirt"
}

// HostRecord specifies a DNS host record for the built-in DHCP-server.
type HostRecord struct {
	IP       string `xml:"ip,attr,omitempty"`
	Hostname string `xml:"hostname,omitempty"`
}

// NetworkAddressing specifies an IP-address for the network, and port range for
// DHCP-server.
type NetworkAddressing struct {
	Address string          `xml:"address,attr,omitempty"`
	NetMask string          `xml:"netmask,attr,omitempty"`
	IPRange *IPAddressRange `xml:"dhcp>range,omitempty"`
}

// Network specifies a virtual network.
type Network struct {
	XMLName xml.Name           `xml:"network"`
	Name    string             `xml:"name"`              // Identifier
	UUID    string             `xml:"uuid,omitempty"`    // UUID
	Forward *NetworkForwarder  `xml:"forward,omitempty"` // NAT forwarder options
	Bridge  *NetworkBridge     `xml:"bridge,omitempty"`  // Bridge device options
	MAC     *MAC               `xml:"mac,omitempty"`
	Hosts   []HostRecord       `xml:"dns>host,omitempty"`
	IP      *NetworkAddressing `xml:"ip,omitempty"`
}

// FilterRule specifies a rule for filtering.
// It is only possible to use one type of conditions.
type FilterRule struct {
	Action       string        `xml:"action,attr,omitempty"`     // "drop", "accept", "return", "continue"
	Direction    string        `xml:"direction,attr,omitempty"`  // "in", "out", "inout"
	Priority     int           `xml:"priority,attr,omitempty"`   // [-1000; 1000]
	StateMatch   string        `xml:"statematch,attr,omitempty"` // "false", "true" (default)
	IPConditions []IPCondition `xml:"ip,omitempty"`
	// TODO: Add support for other condition types
}

// IPCondition specifies a condition on API packages used for filtering.
type IPCondition struct {
	SourceMACAddress      string `xml:"srcmacaddr,attr,omitempty"`
	SourceMACMask         string `xml:"srcmacmask,attr,omitempty"`
	DestinationMACAddress string `xml:"dstmacaddr,attr,omitempty"`
	DestinationMACMask    string `xml:"dstmacmask,attr,omitempty"`
	SourceIPAddress       string `xml:"srcipaddr,attr,omitempty"`
	SourceIPMask          string `xml:"srcipmask,attr,omitempty"`
	DestinationIPAddress  string `xml:"dstipaddr,attr,omitempty"`
	DestinationIPMask     string `xml:"dstipmask,attr,omitempty"`
	Protocol              string `xml:"protocol,attr,omitempty"`
	SourcePortStart       int    `xml:"srcportstart,attr,omitempty"`
	SourcePortEnd         int    `xml:"srcportend,attr,omitempty"`
	DestinationPortStart  int    `xml:"dstportstart,attr,omitempty"`
	DestinationPortEnd    int    `xml:"dstportend,attr,omitempty"`
}

// FilterChain specifies a network-filter chain.
type FilterChain struct {
	Name     string            `xml:"name,attr,omitempty"`  // Identifier
	UUID     string            `xml:"uuid,omitempty"`       // UUID
	Chain    string            `xml:"chain,attr,omitempty"` // "root", "ipv4", ...
	Priority int               `xml:"priority,attr,omitempty"`
	Rules    []FilterRule      `xml:"rule,omitempty"`
	Filters  []FilterReference `xml:"filterref,omitempty"`
}
