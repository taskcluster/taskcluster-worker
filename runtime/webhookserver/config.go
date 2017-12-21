package webhookserver

import (
	"net"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var localhostConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"provider": schematypes.StringEnum{Options: []string{"localhost"}},
	},
	Required: []string{"provider"},
}

var localtunnelConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"provider": schematypes.StringEnum{Options: []string{"localtunnel"}},
		"baseUrl":  schematypes.URI{},
	},
	Required: []string{"provider"},
}

var webhooktunnelConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"provider": schematypes.StringEnum{Options: []string{"webhooktunnel"}},
	},
	Required: []string{"provider"},
}

var statelessDNSConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"provider": schematypes.StringEnum{Options: []string{"stateless-dns"}},
		"serverIp": schematypes.String{},
		"serverPort": schematypes.Integer{
			Minimum: 0,
			Maximum: 65535,
		},
		"networkInterface": schematypes.String{
			Description: util.Markdown(`
				Network device webhookserver should listen on. If not supplied, it
				binds to the interface from 'serverIp' address
			`),
		},
		"exposedPort": schematypes.Integer{
			Description: util.Markdown(`
				Port webhookserver should listen on. If not supplied, it uses the
				'serverPort' value.
			`),
			Minimum: 0,
			Maximum: 65535,
		},
		"tlsCertificate":     schematypes.String{},
		"tlsKey":             schematypes.String{},
		"statelessDNSSecret": schematypes.String{},
		"statelessDNSDomain": schematypes.String{},
		"expiration": schematypes.Duration{
			Title: "URL Expiration",
			Description: util.Markdown(`
				Used to limit the time period for which the DNS server will return
				an IP for the given worker hostname.

				This should be larger than the maximum task runtime. If not set it'll
				default to 1 day, which is sane for most use-cases.
			`),
		},
	},
	Required: []string{
		"provider",
		"serverIp",
		"serverPort",
		"statelessDNSSecret",
		"statelessDNSDomain",
	},
}

// ConfigSchema specifies schema for configuration passed to NewServer.
var ConfigSchema schematypes.Schema = schematypes.OneOf{
	localhostConfigSchema,
	localtunnelConfigSchema,
	statelessDNSConfigSchema,
	webhooktunnelConfigSchema,
}

// Server abstracts various WebHookServer implementations
type Server interface {
	WebHookServer
	Stop()
}

// NewServer returns a Server implementing WebHookServer, choosing the
// implemetation based on the configuration passed in.
// Config passed must match ConfigSchema.
// Credentials are required if the WebhookServer is Webhooktunnel.
func NewServer(config interface{}, credentials *tcclient.Credentials) (Server, error) {
	var c struct {
		Provider           string        `json:"provider"`
		ServerIP           string        `json:"serverIp"`
		ServerPort         int           `json:"serverPort"`
		NetworkInterface   string        `json:"networkInterface"`
		ExposedPort        int           `json:"exposedPort"`
		TLSCertificate     string        `json:"tlsCertificate"`
		TLSKey             string        `json:"tlsKey"`
		StatelessDNSSecret string        `json:"statelessDNSSecret"`
		StatelessDNSDomain string        `json:"statelessDNSDomain"`
		Expiration         time.Duration `json:"expiration"`
		BaseURL            string        `json:"baseUrl"`
		ProxyURL           string        `json:"proxyUrl"`
	}
	schematypes.MustValidate(ConfigSchema, config)
	if schematypes.MustMap(localhostConfigSchema, config, &c) == nil {
		return NewTestServer()
	}
	if schematypes.MustMap(localtunnelConfigSchema, config, &c) == nil {
		return NewLocalTunnel(c.BaseURL)
	}
	if schematypes.MustMap(webhooktunnelConfigSchema, config, &c) == nil {
		s, err := NewWebhookTunnel(credentials)
		return s, err
	}
	if schematypes.MustMap(statelessDNSConfigSchema, config, &c) == nil {
		s, err := NewLocalServer(
			net.ParseIP(c.ServerIP), c.ServerPort,
			c.NetworkInterface, c.ExposedPort,
			c.StatelessDNSDomain,
			c.StatelessDNSSecret,
			c.TLSCertificate,
			c.TLSKey,
			c.Expiration,
		)
		if err == nil {
			go s.ListenAndServe()
		}
		return s, err
	}
	panic("Invalid config shouldn't be valid")
}
