package vm

import (
	"bufio"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/go-qemu"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/fsnotify/fsnotify"
	pnm "github.com/jbuchbinder/gopnm"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

const (
	vncSocketFile = "vnc.sock"
	qmpSocketFile = "qmp.sock"
)

// LinuxBootOptions holds optionals boot options for Linux.
// These are exclusively useful for building images and should not be used in
// production when running per-task VMs. But they can greatly simplify image
// building by facilitating injection of arguments for the Linux kernel.
type LinuxBootOptions struct {
	Kernel string // -kernel <bzImage>
	Append string // -append <cmdline>
	Initrd string // -initrd <file>
}

// VirtualMachine holds the QEMU process and associated resources.
// This is useful as the VM remains alive in the ResultSet stage, as we use
// guest tools to copy files from the virtual machine.
type VirtualMachine struct {
	m            sync.Mutex // Protect access to resources
	started      bool
	network      Network
	image        Image
	socketFolder string
	qemu         *exec.Cmd
	qemuDone     chan<- struct{}
	Done         <-chan struct{} // Closed when the virtual machine is done
	Error        error           // Error, to be read after Done is closed
	monitor      runtime.Monitor
	domain       *qemu.Domain
}

// NewVirtualMachine constructs a new virtual machine using the given
// machineOptions, image, network and cdroms.
//
// Returns engines.MalformedPayloadError if machineOptions and image definition
// are conflicting. If this returns an error, caller is responsible for
// releasing all resources, otherwise, they will be held by the VirtualMachine
// object.
func NewVirtualMachine(
	limits MachineLimits,
	image Image, network Network, socketFolder, cdrom1, cdrom2 string,
	bootOptions LinuxBootOptions,
	monitor runtime.Monitor,
) (*VirtualMachine, error) {
	// Get machine definition and set defaults
	m, err := image.Machine().WithDefaults(defaultMachine).ApplyLimits(limits)
	if err != nil {
		return nil, err
	}
	o := m.options

	// Create a sub-folder in the socketFolder
	socketFolder = filepath.Join(socketFolder, slugid.Nice())

	// Construct virtual machine
	vm := &VirtualMachine{
		socketFolder: socketFolder,
		network:      network,
		image:        image,
		monitor:      monitor,
	}

	vncSocket := filepath.Join(vm.socketFolder, vncSocketFile)
	qmpSocket := filepath.Join(vm.socketFolder, qmpSocketFile)

	// Construct options for QEMU
	var options []string
	// Auxiliary functions for defining options
	type args map[string]string
	option := func(option, prefix string, args args) {
		// Sort for consistency. QEMU shouldn't care about order, but if there is
		// a bug it's nice that it's consistent.
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Create pairs and join with a comma
		pairs := make([]string, len(keys))
		for i, k := range keys {
			pairs[i] = fmt.Sprintf("%s=%s", k, args[k])
		}
		// Preprend with prefix, if it's non-empty
		if prefix != "" {
			pairs = append([]string{prefix}, pairs...)
		}
		options = append(options, "-"+option, strings.Join(pairs, ","))
	}
	device := func(device string, args args) { option("device", device, args) }
	drive := func(flags string, args args) { option("drive", flags, args) }

	// Set options always required
	options = append(options,
		"-S",              // Wait for QMP command "continue" before starting execution
		"-no-user-config", // Don't load user config
		"-nodefaults",     // Don't apply any default values
		"-name", "qemu-guest",
		"-cpu", strings.Join(append([]string{o.CPU}, o.Flags...), ","),
		"-m", strconv.Itoa(o.Memory),
		"-uuid", o.UUID,
		"-k", o.KeyboardLayout,
	)

	if bootOptions.Kernel != "" {
		option("kernel", bootOptions.Kernel, nil)
	}
	if bootOptions.Append != "" {
		option("append", bootOptions.Append, nil)
	}
	if bootOptions.Initrd != "" {
		option("initrd", bootOptions.Initrd, nil)
	}

	option("boot", "", args{
		"menu":   "off",
		"strict": "on",
	})
	option("realtime", "", args{
		"mlock": "off", // TODO: Enable for things like talos
	})
	option("rtc", "", args{
		"base": "utc", // TODO: Allow clock=vm for loadvm with windows
	})
	option("smp", "", args{
		"cpus":    strconv.Itoa(o.Threads * o.Cores * o.Sockets),
		"threads": strconv.Itoa(o.Threads), // threads per core
		"cores":   strconv.Itoa(o.Cores),   // cores per socket
		"sockets": strconv.Itoa(o.Sockets), // sockets in the machine
		// TODO: fit to system HT, see: https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-devices-system-cpu
	})
	option("machine", o.Chipset, args{
		"accel": "kvm",
		// TODO: Configure additional options
	})
	option("vnc", "unix:"+vncSocket, args{
		"share": "force-shared",
	})

	// QMP monitoring socket
	option("chardev", "socket,server,nowait", args{
		"id":   "qmpsocket",
		"path": qmpSocket,
	})
	option("mon", "", args{
		"chardev": "qmpsocket",
		"mode":    "control",
	})

	// Graphics
	device(o.Graphics, args{
		"id":   "video-0",
		"bus":  "pci.0",
		"addr": "0x2", // QEMU uses PCI 0x2 for VGA by default
	})

	// USB
	device(o.USB, args{
		"id":   "usb",
		"bus":  "pci.0",
		"addr": "0x3", // Always put USB on PCI 0x3
	})

	// Virtio ballon device
	device("virtio-balloon-pci", args{
		"id":   "balloon-0",
		"bus":  "pci.0",
		"addr": "0x4", // Always put balloon on PCI 0x4
	})

	// Network
	option("netdev", vm.network.NetDev("netdev-0"), nil)
	device(o.Network, args{
		"netdev": "netdev-0",
		"id":     "nic0",
		"mac":    o.MAC,
		"bus":    "pci.0",
		"addr":   "0x5", // Always put network on PCI 0x5
	})

	// Reserve PCI 0x6 for sound device/controller
	if o.Keyboard == "usb-kbd" {
		device("usb-kbd", args{
			"id":   "keyboard-0",
			"bus":  "usb.0",
			"port": "1", // USB port offset starts at 1
		})
	}
	if o.Mouse == "usb-mouse" {
		device("usb-mouse", args{
			"id":   "mouse-0",
			"bus":  "usb.0",
			"port": "2",
		})
	}
	if o.Tablet == "usb-tablet" {
		device("usb-tablet", args{
			"id":   "tablet-0",
			"bus":  "usb.0",
			"port": "3",
		})
	}

	// Storage
	drive("", args{
		"file":   vm.image.DiskFile(),
		"if":     "none",
		"id":     "boot-disk",
		"cache":  "unsafe", // TODO: Reconsider 'native' w. cache not 'unsafe'
		"aio":    "threads",
		"format": vm.image.Format(),
		"werror": "report",
		"rerror": "report",
	})
	device(o.Storage, args{
		"scsi":      "off",
		"bus":       "pci.0",
		"addr":      "0x8", // Start disks as 0x8, reserve 0x7 for future
		"drive":     "boot-disk",
		"id":        "virtio-disk0",
		"bootindex": "1",
	})

	// Sound
	if o.Sound != "none" {
		if strings.Contains(o.Sound, "/") {
			sound := strings.Split(o.Sound, "/")
			// Sound Device
			device(sound[0], args{
				"id":  "sound-0-device-0",
				"bus": "sound-0.0",
				"cad": "0",
			})
			// Sound controller
			device(sound[1], args{
				"id":   "sound-0",
				"bus":  "pci.0",
				"addr": "0x6", // Always put sound on PCI 0x6
			})
		} else {
			// PCI Sound device
			device(o.Sound, args{
				"id":   "sound-0",
				"bus":  "pci.0",
				"addr": "0x6", // Always put sound on PCI 0x6
			})
		}
	}

	// CD drives for qemu-build
	if cdrom1 != "" {
		drive("readonly", args{
			"file":   cdrom1,
			"if":     "none",
			"id":     "cdrom1",
			"cache":  "unsafe",
			"aio":    "threads", // TODO: Reconsider 'native' w. cache not 'unsafe'
			"format": "raw",
			"werror": "report",
			"rerror": "report",
		})
		device("ide-cd", args{
			"bootindex": "2",
			"drive":     "cdrom1",
			"id":        "ide-cd1",
			"bus":       "ide.0",
			"unit":      "0",
		})
	}
	if cdrom2 != "" {
		drive("readonly", args{
			"file":   cdrom2,
			"if":     "none",
			"id":     "cdrom2",
			"cache":  "unsafe",
			"aio":    "threads", // TODO: Reconsider 'native' w. cache not 'unsafe'
			"format": "raw",
			"werror": "report",
			"rerror": "report",
		})
		device("ide-cd", args{
			"bootindex": "3",
			"drive":     "cdrom2",
			"id":        "ide-cd2",
			"bus":       "ide.0",
			"unit":      "1",
		})
	}

	// Create done channel
	qemuDone := make(chan struct{})
	vm.qemuDone = qemuDone
	vm.Done = qemuDone

	// Create QEMU process
	vm.qemu = exec.Command("qemu-system-x86_64", options...)

	return vm, nil
}

// SetHTTPHandler sets the HTTP handler for the meta-data service.
func (vm *VirtualMachine) SetHTTPHandler(handler http.Handler) {
	vm.m.Lock()
	defer vm.m.Unlock()
	if vm.network != nil {
		// Ignore the case where network has been released
		vm.network.SetHandler(handler)
	}
}

// Start the virtual machine.
func (vm *VirtualMachine) Start() {
	vm.m.Lock()
	if vm.started {
		vm.m.Unlock()
		panic("virtual machine instance have already been started once")
	}
	vm.started = true
	vm.m.Unlock()

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()
	vm.qemu.Stdout = stdoutWriter
	vm.qemu.Stderr = stderrWriter

	// Local reference to socketFolder to avoid race condition
	socketFolder := vm.socketFolder

	// Create socket folder
	err := os.MkdirAll(socketFolder, 0700)
	if err != nil {
		vm.monitor.Errorf("Failed to create socketFolder, error: %s", err)
		vm.Error = err
		close(vm.qemuDone)
		return
	}

	// Start monitor socketFolder for vnc and qmp sockets
	socketsReady, err := vm.waitForSockets()
	if err != nil {
		vm.monitor.Errorf("Error configuring socketFolder monitoring, error: %s", err)
		vm.Error = err
		close(vm.qemuDone)
		return
	}

	// Start QEMU
	vm.Error = vm.qemu.Start()
	if vm.Error != nil {
		close(vm.qemuDone)
		return
	}

	// Forward stdout/err to log
	// Normally QEMU won't write anything... So sending everything to log is
	// probably a good thing. Usually, it's errors and deprecation notices.
	go scanLog(stdout, vm.monitor.Info, vm.monitor.Error)
	go scanLog(stderr, vm.monitor.Error, vm.monitor.Error)

	// Wait for QEMU to finish and cleanup
	go func() {
		// Wait for QEMU to be done
		werr := vm.qemu.Wait()
		debug("qemu terminated")

		// Acquire lock
		vm.m.Lock()
		defer vm.m.Unlock()

		// Close output pipes
		stdoutWriter.Close()
		stderrWriter.Close()

		// Set error, if any and not already set
		if vm.Error == nil {
			vm.Error = werr
		}

		// Close domain, if set
		if vm.domain != nil {
			vm.domain.Close()
		}

		// Release network and image
		vm.network.Release()
		vm.network = nil
		vm.image.Release()
		vm.image = nil

		// Remove socket folder
		os.RemoveAll(vm.socketFolder)
		vm.socketFolder = ""

		// Notify everybody that the VM is stopped
		// Ensure resources are freed first, otherwise we'll race with resources
		// against the next task. If the number of resources is limiting the
		// number of concurrent tasks we can run.
		// This is usually the case, so race would happen at full capacity.
		close(vm.qemuDone)
	}()

	// Wait for vncSocket and qmpSocket to appear, or qemu to crash
	select {
	case err = <-socketsReady:
		if err != nil {
			vm.abort(err)
			return
		}
	case <-vm.Done:
		return
	}

	// Create monitor
	qmpSocket := filepath.Join(socketFolder, qmpSocketFile)
	monitor, err := qmp.NewSocketMonitor("unix", qmpSocket, 5*time.Second)
	if err != nil {
		debug("Error opening QMP monitor, error: %s", err)
		vm.abort(fmt.Errorf("Failed to open QMP monitor, error: %s", err))
		return
	}

	if err = monitor.Connect(); err != nil {
		debug("Error connecting QMP monitor, error: %s", err)
		vm.abort(fmt.Errorf("Failed to connect to QMP monitor, error: %s", err))
		monitor.Disconnect()
		return
	}

	domain, err := qemu.NewDomain(monitor, slugid.Nice())
	if err != nil {
		debug("Error creating domain from QMP monitor, error: %s", err)
		vm.abort(fmt.Errorf("Failed to create domain from QMP monitor, error: %s", err))
		monitor.Disconnect()
		return
	}

	// Acquire lock when we set domain, so we don't race with QEMU cleanup code
	// above... This code will close domain, if it's non-nil, so after setting it
	// just check that vm.Done isn't closed as that would indicate the code
	// already ran, and we just have to cleanup.
	vm.m.Lock()
	vm.domain = domain
	select {
	case <-vm.Done:
		vm.domain.Close()
		vm.m.Unlock()
		return
	default:
	}
	vm.m.Unlock()

	// Run QMP command continue to start execution
	_, err = vm.domain.Run(qmp.Command{
		Execute: "cont",
	})
	if err != nil {
		debug("Error executing QMP command 'cont', error: %s", err)
		vm.abort(fmt.Errorf("Failed QMP command 'cont', error: %s", err))
	}
}

// abort kills the VM and sets the error, if it's not already dead with another
// error. This ensure we don't accidentally overwrite vm.Error with an error
// that was the result of the original error.
func (vm *VirtualMachine) abort(err error) {
	vm.m.Lock()
	if vm.Error != nil {
		vm.Error = err
	}
	vm.m.Unlock()
	vm.Kill()
}

// Kill the virtual machine, can only be called after Start()
func (vm *VirtualMachine) Kill() {
	select {
	case <-vm.Done:
		return // We're obviously not running, so we must be done
	default:
		debug("terminating QEMU with SIGKILL")
		vm.qemu.Process.Kill()
		<-vm.Done
	}
}

// VNCSocket returns the path to VNC socket, empty-string if closed.
func (vm *VirtualMachine) VNCSocket() string {
	// Lock access to vncSocket
	vm.m.Lock()
	defer vm.m.Unlock()

	if vm.socketFolder == "" {
		return ""
	}

	return filepath.Join(vm.socketFolder, vncSocketFile)
}

// Screenshot takes a screenshot of the virtual machine screen as is running.
func (vm *VirtualMachine) Screenshot() (image.Image, error) {
	r, err := vm.domain.ScreenDump()
	if err != nil {
		return nil, fmt.Errorf("Error taking screendump, error: %s", err)
	}
	defer r.Close()
	img, err := pnm.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("Error decoding screendump, error: %s", err)
	}
	return img, nil
}

// waitForSockets will monitor socketFolder and return a channel that is closed
// when vncSocketFile and qmpSocketFile have been created.
func (vm *VirtualMachine) waitForSockets() (<-chan error, error) {
	done := make(chan error)

	// Cache socket folder here to avoid race conditions
	socketFolder := vm.socketFolder

	// Setup file monitoring, if there is an error here we panic, this should
	// always be reliable.
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Failed to setup file system monitoring, error: %s", err)
	}
	err = w.Add(socketFolder)
	if err != nil {
		return nil, fmt.Errorf("Failed to monitor socket folder, error: %s", err)
	}

	// Handle events, and close the done channel when sockets are ready
	go func() {
		vncReady := false
		qmpReady := false
		for !vncReady || !qmpReady {
			select {
			case e := <-w.Events:
				debug("fs-event: %s", e)
				if e.Op == fsnotify.Create {
					if e.Name == filepath.Join(socketFolder, vncSocketFile) {
						vncReady = true
					}
					if e.Name == filepath.Join(socketFolder, qmpSocketFile) {
						qmpReady = true
					}
				}
			case <-vm.Done:
				// Stop monitoring if QEMU has crashed
				w.Close()
				return
			case <-time.After(90 * time.Second):
				done <- fmt.Errorf("vnc and qmp sockets didn't show up in 90s")
				w.Close()
				return
			case err := <-w.Errors:
				done <- fmt.Errorf("Error monitoring file system, error: %s", err)
				w.Close()
				return
			}
		}
		w.Close()
		close(done)
	}()

	return done, nil
}

func scanLog(log io.Reader, infoLog, errorLog func(...interface{})) {
	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		infoLog("QEMU: ", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		errorLog("Error reading QEMU log, error: ", err)
	}
}
