package virt

import (
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

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

func TestNewDomain(t *testing.T) {

	// Start
	controlSocket := "/tmp/guestfwd"
	os.Remove(controlSocket)
	//listener, err := net.Listen("unix", controlSocket)
	listener, err := net.Listen("tcp4", "127.0.0.1:2445")
	nilOrPanic(err, "Failed to listen for domain socket: ", controlSocket)
	os.Chmod(controlSocket, 0777)
	go func() {
		defer listener.Close()
		router := http.NewServeMux()
		server := http.Server{Handler: router}
		router.HandleFunc("/v1/command", func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("Got a request!!!")
			w.WriteHeader(200)
			w.Write([]byte("whoami && date && cat /etc/passwd"))
		})
		server.Serve(listener)
	}()
	//controlSocket = "/tmp/control-socket-virt.sock"

	// Connect to libvirt
	conn, err := libvirt.NewVirConnection("qemu:///system")
	nilOrPanic(err, "Failed at connecting...")
	defer conn.CloseConnection()

	err = defineBuiltInNetworkFilters(&conn)
	nilOrPanic(err, "Failed to setup filters")

	// Extract image
	pwd, err := os.Getwd()
	nilOrPanic(err)
	diskPath := path.Join(pwd, "testvm.img")
	err = ExtractImage("tinycore.tar", diskPath)
	nilOrPanic(err, "Failed to extract image")

	disk := Disk{
		Type:      "file",
		Device:    "disk",
		ReadOnly:  false,
		Transient: false,
		Boot:      DefaultBootOrder,
		Source:    DiskSource{Path: diskPath},
		Driver:    DefaultRawDiskDriver,
		Target:    DiskTarget{Device: "vda", Bus: "virtio"},
	}

	network := NetworkInterface{
		Type:   "network",
		Source: NetworkSource{Network: "tc"},
		MAC:    NewMAC(),
		Filter: &FilterReference{Name: "tc-filter", Parameters: []Parameter{
			{Name: "ALLOWED_LOCAL_IP", Value: "192.168.123.1"},
		}},
	}

	vnc := GraphicsDevice{
		Type:        "vnc",
		Listen:      "127.0.0.1",
		Port:        15900,
		AutoPort:    "no",
		SharePolicy: "force-shared",
	}

	domain := Domain{
		Type:              "kvm",
		Name:              "test-vm",
		Memory:            Memory{Size: 256, Unit: "MiB"},
		CurrentMemory:     Memory{Size: 256, Unit: "MiB"},
		VCPU:              VCPU{Maximum: 2, Placement: "static"},
		OSType:            OSType{Type: "hvm", Architecture: "x86_64", Machine: "pc-i440fx-2.1"},
		BootMenu:          DefaultBootMenu,
		PAE:               false,
		ACPI:              false,
		APIC:              false,
		Clock:             DefaultClock,
		OnPowerOff:        "destroy",
		OnReboot:          "destroy",
		OnCrash:           "destroy",
		OnLockFailure:     "poweroff",
		Emulator:          "/usr/bin/kvm",
		Disks:             []Disk{disk},
		Controllers:       DefaultControllers,
		NetworkInterfaces: []NetworkInterface{network},
		InputDevices:      []InputDevice{DefaultMouseDevice, DefaultKeyboardDevice},
		GraphicsDevices:   []GraphicsDevice{vnc},
		VideoDevices:      []VideoDevice{DefaultVideoDevice},
		SoundDevices:      []SoundDevice{DefaultSoundDevice},
	}

	xmlConfig, err := xml.MarshalIndent(domain, "", "  ")
	nilOrPanic(err, "Failed to seralize xml")
	fmt.Println(string(xmlConfig))

	// Define the virtual machine
	dom, err := conn.DomainDefineXML(string(xmlConfig))
	nilOrPanic(err, "Failed to define domain")

	// Start the virtual machine
	err = dom.Create()
	nilOrPanic(err, "Failed to create domain")

	for {
		active, err := dom.IsActive()
		nilOrPanic(err, "Failed to get IsActive")
		if !active {
			break
		}
		time.Sleep(10 * time.Second)
	}
}

//

//

//

//
