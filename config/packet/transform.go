// Package configpacket implements a TransformationProvider that replaces
// objects on the form: {$packet: "VARIABLE"} with a value loaded
// from https://metadata.packet.net/metadata, following VARIABLE values
// are supported:
//   - instance-id
//   - hostname
//   - facility
//   - instance-type
//   - public-ipv4
//   - public-ipv6
//
// If configuration property 'packetMetaDataUrl' this will be used instead of
// 'https://metadata.packet.net/metadata'. This is useful for testing.
package configpacket

import (
	"encoding/json"
	"errors"
	"fmt"

	got "github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-worker/config"
)

const defaultPacketMetaDataURL = "https://metadata.packet.net/metadata"

type provider struct{}

type metadata struct {
	InstanceID   string `json:"id"`
	Hostname     string `json:"hostname"`
	Facility     string `json:"facility"`
	InstanceType string `json:"plan"`
	Network      struct {
		Addresses []struct {
			Family  int    `json:"address_family"`
			Public  bool   `json:"public"`
			Address string `json:"address"`
		} `json:"addresses"`
	} `json:"network"`
}

func init() {
	config.Register("packet", provider{})
}

func (provider) Transform(cfg map[string]interface{}) error {
	metaURL, ok := cfg["packetMetaDataUrl"].(string)
	if !ok {
		metaURL = defaultPacketMetaDataURL
	}

	var data metadata

	return config.ReplaceObjects(cfg, "packet", func(val map[string]interface{}) (interface{}, error) {
		// Fetch packet metadata, if not already done
		if data.InstanceID == "" {
			g := got.New()
			res, err := g.Get(metaURL).Send()
			if err != nil {
				return nil, fmt.Errorf("Failed to fetch packet metadata, error: %s", err)
			}
			err = json.Unmarshal(res.Body, &data)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse packet metadata, error: %s", err)
			}
			if data.InstanceID == "" {
				return nil, errors.New("Packet metadata isn't valid missing 'id' property")
			}
		}

		key := val["$packet"].(string)
		switch key {
		case "instance-id":
			return data.InstanceID, nil
		case "hostname":
			return data.Hostname, nil
		case "facility":
			return data.Facility, nil
		case "instance-type":
			return data.InstanceType, nil
		case "public-ipv4":
			for _, addr := range data.Network.Addresses {
				if addr.Family == 4 && addr.Public {
					return addr.Address, nil
				}
			}
			return nil, ErrNoPublicIPv4Address
		case "public-ipv6":
			for _, addr := range data.Network.Addresses {
				if addr.Family == 6 && addr.Public {
					return addr.Address, nil
				}
			}
			return nil, ErrNoPublicIPv6Address
		default:
			return nil, fmt.Errorf("Unknown $packet variable: %s", key)
		}
	})
}
