package virt

import (
	"encoding/xml"
	"fmt"
	"testing"

	"github.com/rgbkrk/libvirt-go"
)

func fmtPanic(a ...interface{}) {
	panic(fmt.Sprintln(a...))
}

func nilOrPanic(err error, a ...interface{}) {
	if err != nil {
		fmtPanic(append(a, err)...)
	}
}

func evalNilOrPanic(f func() error, a ...interface{}) {
	nilOrPanic(f(), a...)
}

func assert(condition bool, a ...interface{}) {
	if !condition {
		fmtPanic(a...)
	}
}

func TestUnmarshalXML(t *testing.T) {
	raw := `
    <domain type='kvm'>
      <name>lubuntu</name>
      <uuid>abc81f9c-99d3-5fe3-ee6e-f1c95212a94a</uuid>
      <memory unit='KiB'>2097152</memory>
      <currentMemory unit='KiB'>2097152</currentMemory>
      <vcpu placement='static'>2</vcpu>
      <os>
        <type arch='x86_64' machine='pc-i440fx-2.1'>hvm</type>
        <boot dev='hd'/>
				<bootmenu enable='no'/>
      </os>
      <features>
        <acpi/>
        <apic/>
        <pae/>
      </features>
      <clock offset='utc'/>
      <on_poweroff>destroy</on_poweroff>
      <on_reboot>restart</on_reboot>
      <on_crash>restart</on_crash>
      <devices>
        <emulator>/usr/bin/kvm</emulator>
        <disk type='file' device='disk'>
          <driver name='qemu' type='raw'/>
          <source file='/home/jonasfj/Mozilla/virt-playground/vm.img'/>
          <target dev='vda' bus='virtio'/>
          <address type='pci' domain='0x0000' bus='0x00' slot='0x05' function='0x0'/>
        </disk>
        <controller type='usb' index='0'>
          <address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x2'/>
        </controller>
        <controller type='pci' index='0' model='pci-root'/>
        <controller type='ide' index='0'>
          <address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x1'/>
        </controller>
        <interface type='network'>
          <mac address='52:54:00:19:a4:6f'/>
          <source network='default'/>
          <model type='virtio'/>
          <address type='pci' domain='0x0000' bus='0x00' slot='0x03' function='0x0'/>
        </interface>
        <input type='mouse' bus='ps2'/>
        <input type='keyboard' bus='ps2'/>
        <graphics type='vnc' port='-1' autoport='yes'>
					<listen address="localhost"/>
				</graphics>
        <sound model='ich6'>
          <address type='pci' domain='0x0000' bus='0x00' slot='0x04' function='0x0'/>
        </sound>
        <video>
          <model type='qxl' ram='65536' vram='65536' heads='1'/>
          <address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x0'/>
        </video>
        <memballoon model='virtio'>
          <address type='pci' domain='0x0000' bus='0x00' slot='0x06' function='0x0'/>
        </memballoon>
      </devices>
    </domain>
  `
	var domain Domain
	err := xml.Unmarshal([]byte(raw), &domain)
	nilOrPanic(err, "Failed to parse xml")
	fmt.Printf("%+v\n", domain)

	rawXML, err := xml.MarshalIndent(domain, "", "  ")
	nilOrPanic(err, "Failed to serialize xml")
	fmt.Println(string(rawXML))
}

func TestNewDomain(t *testing.T) {
	conn, err := libvirt.NewVirConnection("qemu:///system")
	nilOrPanic(err, "Failed at connecting...")
	defer conn.CloseConnection()

}

//

//

//

//
