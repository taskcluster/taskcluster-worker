package network

import (
	"fmt"
	"strconv"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
)

/*
https://en.wikipedia.org/wiki/SRV_record

https://activedirectory.ncsu.edu/services/base-infrastructure/kms/auto-discovery/
http://www.thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html

*/

type kmsConfig struct {
	Username               string        `json:"username"`
	Password               string        `json:"password"`
	Cipher                 string        `json:"cipher"`
	Remote                 string        `json:"remote"`
	Protocol               string        `json:"protocol"`    // udp, tcp-client
	Compression            string        `json:"compression"` // lzo, lz4, none, undefined (means adaptive)
	CertificateAuthority   string        `json:"certificateAuthority"`
	Certificate            string        `json:"certificate"`
	Key                    string        `json:"key"`
	TLS                    bool          `json:"tls"`
	TLSKey                 string        `json:"tlsKey"`
	KeyDirection           *int          `json:"keyDirection"`
	X509Name               string        `json:"x509Name"`
	X509NameType           string        `json:"x509NameType"` // name, name-prefix, subject
	RenegotiationDelay     time.Duration `json:"renegotiationDelay"`
	RemoteExtendedKeyUsage string        `json:"remoteExtendedKeyUsage"`
	Route                  string        `json:"route"`
}

var kmsConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"username":               schematypes.String{},
		"password":               schematypes.String{},
		"cipher":                 schematypes.StringEnum{},
		"remote":                 schematypes.String{},
		"protocol":               schematypes.StringEnum{},
		"compression":            schematypes.StringEnum{},
		"certificateAuthority":   schematypes.String{},
		"certificate":            schematypes.String{},
		"key":                    schematypes.String{},
		"tls":                    schematypes.Boolean{},
		"tlsKey":                 schematypes.String{},
		"keyDirection":           schematypes.Integer{},
		"x509Name":               schematypes.String{},
		"x509NameType":           schematypes.StringEnum{},
		"renegotiationDelay":     schematypes.Duration{},
		"remoteExtendedKeyUsage": schematypes.String{},
		"route":                  schematypes.String{},
	},
	Required: []string{
		"remote", "cipher", "protocol", "route",
	},
}

func (k *kmsConfig) Arguments() []string {
	var args []string
	arg := func(a string, opts ...string) {
		args = append(args, fmt.Sprintf("--%s", a))
		args = append(args, opts...)
	}

	// Client options
	if k.Username != "" && k.Password != "" {
		// TODO: Write these to a file
		arg("auth-user-pass", "<file-with-user/pass>")
	}
	arg("auth-retry", "none")
	arg("pull")

	// Encryption options
	arg("cipher", k.Cipher)

	// Tunnel options
	arg("remote", k.Remote)
	arg("resolv-retry", "infinite")
	arg("proto", k.Protocol)
	arg("nobind")
	if k.Compression != "" {
		if k.Compression == "none" {
			arg("compress", "")
		}
		arg("compress", k.Compression)
	}

	// Tun device
	arg("dev", "kmstun")
	arg("dev-type", "tun")

	// Drop permissions
	arg("user", "nobody")
	arg("group", "nobody")

	// Persist key material
	arg("persist-key")
	arg("persist-tun")

	// TLS Mode
	if k.CertificateAuthority != "" {
		// TODO: write to file
		arg("ca", "<file-w-ca>")
	}
	if k.Certificate != "" {
		// TODO: write to file
		arg("cert", "<file-w-cert>")
	}
	if k.Key != "" {
		// TODO: write to file
		arg("key", "<file-w-key>")
	}
	if k.TLS {
		arg("tls-client")
	}
	if k.TLSKey != "" {
		// TODO: write to file
		arg("tls-auth", "<file-w-tls-key>")
	}
	if k.KeyDirection != nil {
		arg("key-direction", strconv.Itoa(*k.KeyDirection))
	}
	if k.X509Name != "" {
		if k.X509NameType != "" {
			arg("verify-x509-name", k.X509Name, k.X509NameType)
		} else {
			arg("verify-x509-name", k.X509Name)
		}
	}
	if k.RenegotiationDelay != 0 {
		arg("reneg-sec", strconv.Itoa(int(k.RenegotiationDelay.Seconds())))
	}
	if k.RemoteExtendedKeyUsage != "" {
		arg("remote-cert-eku", k.RemoteExtendedKeyUsage)
	}

	// Routing
	arg("route-nopull")
	arg("route", k.Route)

	// Error messages
	arg("verb", "0")
	arg("errors-to-stderr")

	return args
}
