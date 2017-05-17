package openvpn

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

/*
https://en.wikipedia.org/wiki/SRV_record

https://activedirectory.ncsu.edu/services/base-infrastructure/kms/auto-discovery/
http://www.thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html

*/

// Config options for an openvpn client
type config struct {
	Username               string        `json:"username"`
	Password               string        `json:"password"`
	Cipher                 string        `json:"cipher"`
	Remote                 string        `json:"remote"`
	Port                   int           `json:"port"`
	Protocol               string        `json:"protocol"`
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
	Routes                 []string      `json:"routes"`
}

// ConfigSchema for options given to New()
var ConfigSchema schematypes.Schema = schematypes.Object{
	Properties: schematypes.Properties{
		"username": schematypes.String{},
		"password": schematypes.String{},
		"cipher": schematypes.StringEnum{
			Title: "Encryption Cipher",
			Options: []string{ // openvpn --show-ciphers (excluded legacy ciphers)
				"AES-128-CBC", "AES-128-CFB", "AES-128-CFB1", "AES-128-CFB8", "AES-128-GCM", "AES-128-OFB",
				"AES-192-CBC", "AES-192-CFB", "AES-192-CFB1", "AES-192-CFB8", "AES-192-GCM", "AES-192-OFB",
				"AES-256-CBC", "AES-256-CFB", "AES-256-CFB1", "AES-256-CFB8", "AES-256-GCM", "AES-256-OFB",
				"CAMELLIA-128-CBC", "CAMELLIA-128-CFB", "CAMELLIA-128-CFB1", "CAMELLIA-128-CFB8", "CAMELLIA-128-OFB",
				"CAMELLIA-192-CBC", "CAMELLIA-192-CFB", "CAMELLIA-192-CFB1", "CAMELLIA-192-CFB8", "CAMELLIA-192-OFB",
				"CAMELLIA-256-CBC", "CAMELLIA-256-CFB", "CAMELLIA-256-CFB1", "CAMELLIA-256-CFB8", "CAMELLIA-256-OFB",
				"SEED-CBC", "SEED-CFB", "SEED-OFB",
			},
		},
		"remote": schematypes.String{
			Title: "Remote Host",
			Description: util.Markdown(`
				Remote host running the VPN server.

				This is the '--remote' argument for openvpn.
			`),
		},
		"port": schematypes.Integer{
			Title:   "Remote Port",
			Minimum: 0,
			Maximum: 65535,
		},
		"protocol": schematypes.StringEnum{
			Title:   "Protocol",
			Options: []string{"udp", "tcp-client"},
		},
		"compression": schematypes.StringEnum{
			Title: "Compression",
			Description: util.Markdown(`
				Compression algorithm to employ, if not given openvpn defaults to
				adaptive mode.
			`),
			Options: []string{"lzo", "lz4", "none"},
		},
		"certificateAuthority": schematypes.String{
			Title: "Certificate Authority",
			Description: util.Markdown(`
				Certificate authority chain as one or more PEM encoded certificates.

				This is the '--ca' argument for openvpn.
			`),
		},
		"certificate": schematypes.String{
			Title: "Certificate",
			Description: util.Markdown(`
				Client certificate as PEM encoded string.

				This is the '--cert' argument for openvpn.
			`),
		},
		"key": schematypes.String{
			Title: "Key",
			Description: util.Markdown(`
				Private key matching the certificate as PEM encoded string.

				This is the '--key' argument for openvpn.
			`),
		},
		"tls": schematypes.Boolean{
			Title: "Enable TLS",
		},
		"tlsKey": schematypes.String{
			Title: "TLS Key",
			Description: util.Markdown(`
				TLS key as PEM encoded string.

				This is the '--tls-auth' argument for openvpn.
			`),
		},
		"keyDirection": schematypes.Integer{
			Title: "Key Direction",
			Description: util.Markdown(`
				Key direction for TLS, this is either 0 or 1, opposite of what the
				server is using.

				This is the '--key-direction' argument for openvpn.
			`),
			Minimum: 0,
			Maximum: 1,
		},
		"x509Name": schematypes.String{
			Title: "x509 Name",
			Description: util.Markdown(`
				Expected x509 name of remote server.

				This is the first openvpn argument for '--verify-x509-name'.
			`),
		},
		"x509NameType": schematypes.StringEnum{
			Title: "x509 Name Type",
			Description: util.Markdown(`
				Type of the name given in 'x509Name'.

				This is the second openvpn argument for '--verify-x509-name'.
			`),
			Options: []string{"name", "name-prefix", "subject"},
		},
		"renegotiationDelay": schematypes.Duration{
			Title: "Renegotiation Delay",
			Description: util.Markdown(`
				Time to renegotiation of data channel.

				This is the '--reneg-sec' argument for openvpn.
			`),
		},
		"remoteExtendedKeyUsage": schematypes.String{
			Title: "Require Extended Key Usage",
			Description: util.Markdown(`
				Require that server certificate is signed with the explicit
				**extended key usage** given here in _oid_ notation.

				This is the '--remote-cert-eku' argument for openvpn.
			`),
		},
		"routes": schematypes.Array{
			Title: "Routes",
			Items: schematypes.String{
				Pattern: `^\d+(\.\d+){3}$`,
				Title:   "Route",
				Description: util.Markdown(`
  				Route to be exposed, this must be an IPv4 address.

  				This is the '--route' argument for openvpn.
  			`),
			},
			Unique: true,
		},
	},
	Required: []string{
		"remote", "cipher", "protocol", "routes",
	},
}
