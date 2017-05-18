package openvpn

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// A VPN object wraps a running instance of openvpn.
type VPN struct {
	cmd          *exec.Cmd
	config       config
	monitor      runtime.Monitor
	folder       runtime.TemporaryFolder
	stdoutWriter io.WriteCloser
	stderrWriter io.WriteCloser
	deviceName   string
	routes       []net.IP
	resolved     atomics.Once
	waitErr      error
	disposed     atomics.Once
}

// Options for creating a new VPN client with New().
type Options struct {
	DeviceName       string // Name for TUN device
	Config           interface{}
	Monitor          runtime.Monitor
	TemporaryStorage runtime.TemporaryStorage
}

// New creates a new VPN client given configuration that matches ConfigSchema.
func New(options Options) (*VPN, error) {
	var c config
	schematypes.MustValidateAndMap(ConfigSchema, options.Config, &c)

	// Ensure that we have a device name
	if options.DeviceName == "" {
		panic("A TUN DeviceName must be given in openvpn.Options")
	}

	// Create a temporary folder for storing keyfiles...
	folder, err := options.TemporaryStorage.NewFolder()
	if err != nil {
		return nil, errors.Wrap(err, "error creating VPN client, couldn't create temporary folder")
	}

	vpn := &VPN{
		config:     c,
		monitor:    options.Monitor,
		folder:     folder,
		deviceName: options.DeviceName,
	}

	// Create file with username/password
	var userPassFile string
	if c.Username != "" && c.Password != "" {
		userPassFile = folder.NewFilePath()
		err = ioutil.WriteFile(userPassFile, []byte(
			fmt.Sprintf("%s\n%s\n", c.Username, c.Password),
		), 0600)
		if err != nil {
			folder.Remove()
			return nil, errors.Wrap(err, "error creating VPN client, failed to create temporary file")
		}
	}

	// Create files with CA, cert, key and tls-key
	var caFile, certFile, keyFile, tlsKeyFile string
	if c.CertificateAuthority != "" {
		caFile = folder.NewFilePath()
		if err = ioutil.WriteFile(caFile, []byte(c.CertificateAuthority), 0600); err != nil {
			folder.Remove()
			return nil, errors.Wrap(err, "error creating VPN client, failed to create temporary file")
		}
	}
	if c.Certificate != "" {
		certFile = folder.NewFilePath()
		if err = ioutil.WriteFile(certFile, []byte(c.Certificate), 0600); err != nil {
			folder.Remove()
			return nil, errors.Wrap(err, "error creating VPN client, failed to create temporary file")
		}
	}
	if c.Key != "" {
		keyFile = folder.NewFilePath()
		if err = ioutil.WriteFile(keyFile, []byte(c.Key), 0600); err != nil {
			folder.Remove()
			return nil, errors.Wrap(err, "error creating VPN client, failed to create temporary file")
		}
	}
	if c.TLSKey != "" {
		tlsKeyFile = folder.NewFilePath()
		if err = ioutil.WriteFile(tlsKeyFile, []byte(c.TLSKey), 0600); err != nil {
			folder.Remove()
			return nil, errors.Wrap(err, "error creating VPN client, failed to create temporary file")
		}
	}

	var args []string
	arg := func(a string, opts ...string) {
		args = append(args, fmt.Sprintf("--%s", a))
		args = append(args, opts...)
	}

	// Client options
	if c.Username != "" && c.Password != "" {
		arg("auth-user-pass", userPassFile)
	}
	arg("auth-retry", "none")
	arg("pull")

	// Encryption options
	arg("cipher", c.Cipher)

	// Tunnel options
	arg("remote", c.Remote)
	if c.Port != 0 {
		arg("rport", strconv.Itoa(c.Port))
	}
	arg("resolv-retry", "infinite")
	arg("proto", c.Protocol)
	arg("nobind")
	if c.Compression != "" {
		if c.Compression == "none" {
			arg("compress", "")
		}
		arg("compress", c.Compression)
	}

	// Tun device
	arg("dev", options.DeviceName)
	arg("dev-type", "tun")

	// Drop permissions
	arg("user", "nobody")
	arg("group", "nogroup")

	// Persist key material
	arg("persist-key")
	arg("persist-tun")

	// TLS Mode
	if c.CertificateAuthority != "" {
		arg("ca", caFile)
	}
	if c.Certificate != "" {
		arg("cert", certFile)
	}
	if c.Key != "" {
		arg("key", keyFile)
	}
	if c.TLS {
		arg("tls-client")
	}
	if c.TLSKey != "" {
		arg("tls-auth", tlsKeyFile)
	}
	if c.KeyDirection != nil {
		arg("key-direction", strconv.Itoa(*c.KeyDirection))
	}
	if c.X509Name != "" {
		if c.X509NameType != "" {
			arg("verify-x509-name", c.X509Name, c.X509NameType)
		} else {
			arg("verify-x509-name", c.X509Name)
		}
	}
	if c.RenegotiationDelay != 0 {
		arg("reneg-sec", strconv.Itoa(int(c.RenegotiationDelay.Seconds())))
	}
	if c.RemoteExtendedKeyUsage != "" {
		arg("remote-cert-eku", c.RemoteExtendedKeyUsage)
	}

	// Routing
	arg("route-nopull")
	for _, route := range c.Routes {
		vpn.routes = append(vpn.routes, net.ParseIP(route))
		arg("route", route)
	}

	// Error messages
	arg("verb", "0")
	arg("errors-to-stderr")

	// Create openvpn process
	vpn.cmd = exec.Command("openvpn", args...)

	// Setup pipes for stdout/stderr
	var stdout, stderr io.Reader
	stdout, vpn.stdoutWriter = io.Pipe()
	stderr, vpn.stderrWriter = io.Pipe()
	vpn.cmd.Stdout = vpn.stdoutWriter
	vpn.cmd.Stderr = vpn.stderrWriter
	monitor := vpn.monitor.WithPrefix("openvpn")
	go scanLog(stdout, monitor.Info, monitor.Error)
	go scanLog(stderr, monitor.Error, monitor.Error)

	debug("starting openvpn")
	err = vpn.cmd.Start()
	if err != nil {
		folder.Remove()
		vpn.stdoutWriter.Close()
		vpn.stderrWriter.Close()
		return nil, errors.Wrap(err, "failed to start openvpn subprocess")
	}

	go vpn.waitForCommand()

	return vpn, nil
}

func (vpn *VPN) waitForCommand() {
	err := vpn.cmd.Wait()
	vpn.resolved.Do(func() {
		if err != nil {
			vpn.waitErr = errors.Wrap(err, "openvpn exited non-zero")
		}
	})
	vpn.disposed.Do(func() {
		vpn.folder.Remove()
		vpn.stdoutWriter.Close()
		vpn.stderrWriter.Close()
	})
}

// Routes exposed by the VPN
func (vpn *VPN) Routes() []net.IP {
	return vpn.routes
}

// DeviceName returns the TUN device name
func (vpn *VPN) DeviceName() string {
	return vpn.deviceName
}

// Wait waits for the openvpn client to exit, return nil if stopped by Stop()
func (vpn *VPN) Wait() error {
	vpn.disposed.Wait()
	return vpn.waitErr
}

// Stop signals the openvpn client to exit
func (vpn *VPN) Stop() {
	vpn.resolved.Do(func() {
		vpn.cmd.Process.Kill()
	})
	vpn.disposed.Wait()
}

func scanLog(log io.Reader, infoLog, errorLog func(...interface{})) {
	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		infoLog("openvpn:", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		errorLog("Error reading openvpn log, error: ", err)
	}
}
