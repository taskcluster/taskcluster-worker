package virt

import (
	"encoding/xml"
	"fmt"

	"github.com/rgbkrk/libvirt-go"
)

// Name of the root network filter to be applied for all VMs
const networkFilterRoot = "tc-worker-filter"

// undefineNetworkFilter deletes a network filter, ignores error if it doesn't
// exist. But otherwise returns error if there is one.
func undefineNetworkFilter(c *libvirt.VirConnection, name string) error {
	filter, err := c.LookupNWFilterByName(name)
	if err == nil {
		err = filter.Undefine()
		if err != nil {
			filter.Free()
			return err
		}
		return filter.Free()
	}
	return nil
}

// defineNetworkFilter creates the filter
func defineNetworkFilter(c *libvirt.VirConnection, filter FilterChain) error {
	xmlConfig, err := xml.MarshalIndent(filter, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("Failed to serialize network-filter XML, error: %s", err))
	}
	f, err := c.NWFilterDefineXML(string(xmlConfig))
	if err != nil {
		return fmt.Errorf("Failed to serialize network-filter XML, error: %s", err)
	}
	err = f.Free()
	if err != nil {
		return fmt.Errorf("Failed to free filter struct, error: %s", err)
	}
	return nil
}

// defineNetworkFilters creates the network filters. Undefines them if they
// exist and then redefines them.
func defineBuiltInNetworkFilters(c *libvirt.VirConnection) error {
	const (
		restrictPackageFromPrivateNetwork = "tc-worker-filter-from-private-ip"
		restrictPackageToPrivateNetwork   = "tc-worker-filter-to-private-ip"
		enforceSourceSubNet               = "tc-worker-enforce-source-subnet"
		enforceDestinationSubNet          = "tc-worker-enforce-destination-subnet"
	)

	// Remove the filters if they already exists
	undefineNetworkFilter(c, networkFilterRoot)
	undefineNetworkFilter(c, restrictPackageFromPrivateNetwork)
	undefineNetworkFilter(c, restrictPackageToPrivateNetwork)
	undefineNetworkFilter(c, enforceSourceSubNet)
	undefineNetworkFilter(c, enforceDestinationSubNet)

	// Disallow out-going IP packages to private IP network
	err := defineNetworkFilter(c, FilterChain{
		Name:     restrictPackageToPrivateNetwork,
		Chain:    "ipv4",
		Priority: -720,
		Rules: []FilterRule{
			{
				Action:    "return",
				Direction: "out",
				Priority:  500,
				IPConditions: []IPCondition{
					{DestinationIPAddress: "$METADATA_IP"},
				},
			}, {
				// Drop outgoing packages to link-local network: 169.254.0.0/16
				// (with exception of the METADATA_IP, obivously)
				Action:    "drop",
				Direction: "out",
				Priority:  510,
				IPConditions: []IPCondition{
					{
						DestinationIPAddress: "169.254.0.0",
						DestinationIPMask:    "255.255.0.0",
					},
				},
			}, {
				// Drop outgoing packages to private network: 192.168.0.0/16
				// Used internally by taskcluster-worker, as each VM has a subnet under
				// this private network space. This restriction is critical isolation of
				// VMs, ensuring that they can't talk to eachother.
				Action:    "drop",
				Direction: "out",
				Priority:  520,
				IPConditions: []IPCondition{
					{
						DestinationIPAddress: "192.168.0.0",
						DestinationIPMask:    "255.255.0.0",
					},
				},
			}, {
				// Drop outgoing packages to private network: 10.0.0.0/8
				// Because some sensitive data-center services might be using it.
				Action:    "drop",
				Direction: "out",
				Priority:  530,
				IPConditions: []IPCondition{
					{
						DestinationIPAddress: "10.0.0.0",
						DestinationIPMask:    "255.0.0.0",
					},
				},
			}, {
				// Drop outgoing packages to private network: 172.16.0.0/12
				// Because some sensitive data-center services might be using it.
				// Also if docker is running on the host it might use it for containers.
				Action:    "drop",
				Direction: "out",
				Priority:  540,
				IPConditions: []IPCondition{
					{
						DestinationIPAddress: "172.16.0.0",
						DestinationIPMask:    "255.240.0.0",
					},
				},
			}, {
				// Drop outgoing packages to loopback device: 128.0.0.0/8
				// This is probably excessive, but hardening can't hurt.
				Action:    "drop",
				Direction: "out",
				Priority:  550,
				IPConditions: []IPCondition{
					{
						DestinationIPAddress: "128.0.0.0",
						DestinationIPMask:    "255.0.0.0",
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	// Disallow all incoming IP packages from private network, except for
	// METADATA_IP (the meta-data service)
	err = defineNetworkFilter(c, FilterChain{
		Name:     restrictPackageFromPrivateNetwork,
		Chain:    "ipv4",
		Priority: -730,
		Rules: []FilterRule{
			{
				Action:    "return",
				Direction: "in",
				Priority:  500,
				IPConditions: []IPCondition{
					{
						SourceIPAddress: "$METADATA_IP",
					},
				},
			}, {
				// Drop incoming packages from link-local network: 169.254.0.0/16
				Action:    "drop",
				Direction: "in",
				Priority:  510,
				IPConditions: []IPCondition{
					{
						SourceIPAddress: "169.254.0.0",
						SourceIPMask:    "255.255.0.0",
					},
				},
			}, {
				// Drop incoming packages from private network: 10.0.0.0/8
				Action:    "drop",
				Direction: "in",
				Priority:  520,
				IPConditions: []IPCondition{
					{
						SourceIPAddress: "10.0.0.0",
						SourceIPMask:    "255.0.0.0",
					},
				},
			}, {
				// Drop incoming packages from private network: 172.16.0.0/12
				Action:    "drop",
				Direction: "in",
				Priority:  530,
				IPConditions: []IPCondition{
					{
						SourceIPAddress: "172.16.0.0",
						SourceIPMask:    "255.240.0.0",
					},
				},
			}, {
				// Drop incoming packages from private network: 192.168.0.0/16
				Action:    "drop",
				Direction: "in",
				Priority:  540,
				IPConditions: []IPCondition{
					{
						SourceIPAddress: "192.168.0.0",
						SourceIPMask:    "255.255.0.0",
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	// Disallow all incoming IP packages except for those going to the subnet
	// given by: NETWORK_GATEWAY and NETWORK_MASK
	err = defineNetworkFilter(c, FilterChain{
		Name:     enforceDestinationSubNet,
		Chain:    "ipv4",
		Priority: -730,
		Rules: []FilterRule{
			{
				Action:    "return",
				Direction: "in",
				Priority:  500,
				IPConditions: []IPCondition{
					{
						DestinationIPAddress: "$NETWORK_GATEWAY",
						DestinationIPMask:    "$NETWORK_MASK",
					},
				},
			}, {
				Action:    "drop",
				Direction: "in",
				Priority:  510,
			},
		},
	})
	if err != nil {
		return err
	}

	// Disallow all out-going packages except those coming from the subnet
	// given by: NETWORK_GATEWAY and NETWORK_MASK
	err = defineNetworkFilter(c, FilterChain{
		Name:     enforceSourceSubNet,
		Chain:    "ipv4",
		Priority: -730,
		Rules: []FilterRule{
			{
				Action:    "return",
				Direction: "out",
				Priority:  500,
				IPConditions: []IPCondition{
					{
						SourceIPAddress: "$NETWORK_GATEWAY",
						SourceIPMask:    "$NETWORK_MASK",
					},
				},
			}, {
				Action:    "drop",
				Direction: "out",
				Priority:  510,
			},
		},
	})
	if err != nil {
		return err
	}

	// Combine filters above with the clean-traffic filter in a single root chain.
	err = defineNetworkFilter(c, FilterChain{
		Name:  networkFilterRoot,
		Chain: "root",
		Filters: []FilterReference{
			{Name: restrictPackageFromPrivateNetwork},
			{Name: restrictPackageToPrivateNetwork},
			{Name: enforceSourceSubNet},
			{Name: enforceDestinationSubNet},
			{Name: "clean-traffic"},
		},
	})
	if err != nil {
		return err
	}

	return nil
}
