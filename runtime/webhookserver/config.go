package webhookserver

import (
	"net"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
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

var statelessDNSConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"provider": schematypes.StringEnum{Options: []string{"stateless-dns"}},
		"serverIp": schematypes.String{},
		"serverPort": schematypes.Integer{
			Minimum: 0,
			Maximum: 65535,
		},
		"networkInterface": schematypes.String{
			MetaData: schematypes.MetaData{
				Description: "Network device webhookserver should listen on. If not supplied, it binds to the interface from serverIp address",
			},
		},
		"exposedPort": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Description: "Port webhookserver should listen on. If not supplied, it uses the serverPort value.",
			},
			Minimum: 0,
			Maximum: 65535,
		},
		"tlsCertificate":     schematypes.String{},
		"tlsKey":             schematypes.String{},
		"statelessDNSSecret": schematypes.String{},
		"statelessDNSDomain": schematypes.String{},
		"maxLifeCycle": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title:       "Maximum lifetime of the worker in seconds",
				Description: "Used to limit the time period for which the DNS server will return an IP for the given worker hostname",
			},
			Minimum: 5 * 60,
			Maximum: 31 * 24 * 60 * 60,
		},
	},
	Required: []string{
		"provider",
		"serverIp",
		"serverPort",
		"statelessDNSSecret",
		"statelessDNSDomain",
		"maxLifeCycle",
	},
}

// ConfigSchema specifies schema for configuration passed to NewServer.
var ConfigSchema schematypes.Schema = schematypes.OneOf{
	localhostConfigSchema,
	localtunnelConfigSchema,
	statelessDNSConfigSchema,
}

// Server abstracts various WebHookServer implementations
type Server interface {
	WebHookServer
	Stop()
}

// NewServer returns a Server implementing WebHookServer, choosing the
// implemetation based on the configuration passed in.
//
// Config passed must match ConfigSchema.
func NewServer(config interface{}) (Server, error) {
	var c struct {
		Provider           string `json:"provider"`
		ServerIP           string `json:"serverIp"`
		ServerPort         int    `json:"serverPort"`
		NetworkInterface   string `json:"networkInterface"`
		ExposedPort        int    `json:"exposedPort"`
		TLSCertificate     string `json:"tlsCertificate"`
		TLSKey             string `json:"tlsKey"`
		StatelessDNSSecret string `json:"statelessDNSSecret"`
		StatelessDNSDomain string `json:"statelessDNSDomain"`
		MaxLifeCycle       int    `json:"maxLifeCycle"`
		BaseURL            string `json:"baseUrl"`
	}
	schematypes.MustValidate(ConfigSchema, config)
	if schematypes.MustMap(localhostConfigSchema, config, &c) == nil {
		return NewTestServer()
	}
	if schematypes.MustMap(localtunnelConfigSchema, config, &c) == nil {
		return NewLocalTunnel(c.BaseURL)
	}
	if schematypes.MustMap(statelessDNSConfigSchema, config, &c) == nil {
		s, err := NewLocalServer(
			net.ParseIP(c.ServerIP), c.ServerPort,
			c.NetworkInterface, c.ExposedPort,
			c.StatelessDNSDomain,
			c.StatelessDNSSecret,
			c.TLSCertificate,
			c.TLSKey,
			time.Duration(c.MaxLifeCycle)*time.Second,
		)
		if err == nil {
			go s.ListenAndServe()
		}
		return s, err
	}
	panic("Invalid config shouldn't be valid")
}
