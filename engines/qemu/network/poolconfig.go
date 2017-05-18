package network

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network/openvpn"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type poolConfig struct {
	Subnets     int           `json:"subnets"`
	VPNs        []interface{} `json:"vpnConnections,omitempty"`
	SRVRecords  []srvRecord   `json:"srvRecords,omitempty"`
	HostRecords []hostRecord  `json:"hostRecords,omitempty"`
}

type srvRecord struct {
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Target   string `json:"target,omitempty"`
	Port     int    `json:"port,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
}

type hostRecord struct {
	Names []string `json:"names"`
	IPv4  string   `json:"ipv4,omitempty"`
	IPv6  string   `json:"ipv6,omitempty"`
}

// PoolConfigSchema is the configuration schema to be satisfied by configuration
// passed to NewPool()
var PoolConfigSchema schematypes.Schema = schematypes.Object{
	Properties: schematypes.Properties{
		"subnets": schematypes.Integer{
			Description: util.Markdown(`
				Number of subnets to creates.
				This determines the maximum number of concurrent VMs the worker can run.

				Each subnet defines a set chains and rules in 'iptables', and thus,
				incurs some kernel overhead. It is recommended to create more subnets
				than needed, but to avoid creating the maximum unless required.
			`),
			Minimum: 1,
			Maximum: 100,
		},
		"vpnConnections": schematypes.Array{
			Title: "VPN Connections",
			Description: util.Markdown(`
				VPN Connections to be setup by the worker and exposed to the virtual
				machines, such that connections can be opened from the virtual machine
				to the routes exposed.

				Note: servers on the VPN will not be able to open incoming connections
				to the virtual machines, as the VMs will sit behind NAT.
			`),
			Items: openvpn.ConfigSchema,
		},
		"srvRecords": schematypes.Array{
			Title: "SRV Records",
			Items: schematypes.Object{
				Title: "SRV Record",
				Description: util.Markdown(`
					SRV record to be inserted in the DNS server advertized to the
					virtual machine. This can be useful for auto-discovery of resources
					located in VPN connections exposed to the VM.

					For details on properties please refer to
					[RFC 2782](https://tools.ietf.org/html/rfc2782).
				`),
				Properties: schematypes.Properties{
					"service":  schematypes.String{},
					"protocol": schematypes.String{},
					"domain":   schematypes.String{},
					"target":   schematypes.String{},
					"port":     schematypes.Integer{Minimum: 0, Maximum: 65535},
					"priority": schematypes.Integer{Minimum: 0, Maximum: 65535},
					"weight":   schematypes.Integer{Minimum: 0, Maximum: 65535},
				},
				Required: []string{
					"service", "protocol", "target", "port",
				},
			},
		},
		"hostRecords": schematypes.Array{
			Title: "Host Records",
			Items: schematypes.Object{
				Title: "Host Record",
				Description: util.Markdown(`
					A and AAAA records to insert in the DNS server advertized to the
					virtual machine.
				`),
				Properties: schematypes.Properties{
					"names": schematypes.Array{Items: schematypes.String{}},
					"ipv4":  schematypes.String{},
					"ipv6":  schematypes.String{},
				},
				Required: []string{"names"},
			},
		},
	},
	Required: []string{"subnets"},
}
