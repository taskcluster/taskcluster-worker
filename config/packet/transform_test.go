package configpacket

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taskcluster/taskcluster-worker/config/configtest"
)

const metadataExample = `{
  "id": "eaab5544-76f6-4fa5-a7b3-6f19be51ecc1",
  "hostname": "test",
  "iqn": "iqn.2016-09.net.packet:device.eaab5544",
  "operating_system": {
    "slug": "ubuntu_16_04_image",
    "distro": "ubuntu",
    "version": "16.04"
  },
  "plan": "baremetal_0",
  "facility": "sjc1",
  "tags": [],
  "ssh_keys": [],
  "network": {
    "bonding": {
      "mode": 5
    },
    "interfaces": [
      {
        "name": "enp0s20f0",
        "mac": "0c:c4:7a:b5:85:f6"
      },
      {
        "name": "enp0s20f1",
        "mac": "0c:c4:7a:b5:85:f7"
      }
    ],
    "addresses": [
      {
        "id": "c3fc028e-ca04-4f06-980d-e0af2a7f4f8d",
        "address_family": 4,
        "netmask": "255.255.255.254",
        "public": true,
        "cidr": 31,
        "management": true,
        "network": "147.75.201.16",
        "address": "147.75.201.17",
        "gateway": "147.75.201.16",
        "href": "/ips/c3fc028e-ca04-4f06-980d-e0af2a7f4f8d"
      },
      {
        "id": "9f3d9e85-1d87-4a61-9950-ebccc55ff8a6",
        "address_family": 6,
        "netmask": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe",
        "public": true,
        "cidr": 127,
        "management": true,
        "network": "2604:1380:1000:e900::",
        "address": "2604:1380:1000:e900::1",
        "gateway": "2604:1380:1000:e900::",
        "href": "/ips/9f3d9e85-1d87-4a61-9950-ebccc55ff8a6"
      },
      {
        "id": "89151f81-38ec-4d48-bf0b-88eadb4e6cd2",
        "address_family": 4,
        "netmask": "255.255.255.254",
        "public": false,
        "cidr": 31,
        "management": true,
        "network": "10.88.114.128",
        "address": "10.88.114.129",
        "gateway": "10.88.114.128",
        "href": "/ips/89151f81-38ec-4d48-bf0b-88eadb4e6cd2"
      }
    ]
  },
  "api_url": "https://metadata.packet.net",
  "phone_home_url": "http://147.75.200.3/phone-home",
  "volumes": []
}`

func TestPacketTransform(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(metadataExample))
	}))
	defer s.Close()

	configtest.Case{
		Transform: "packet",
		Input: map[string]interface{}{
			"instance-id":       map[string]interface{}{"$packet": "instance-id"},
			"hostname":          map[string]interface{}{"$packet": "hostname"},
			"facility":          map[string]interface{}{"$packet": "facility"},
			"instance-type":     map[string]interface{}{"$packet": "instance-type"},
			"public-ipv4":       map[string]interface{}{"$packet": "public-ipv4"},
			"public-ipv6":       map[string]interface{}{"$packet": "public-ipv6"},
			"packetMetaDataUrl": s.URL,
		},
		Result: map[string]interface{}{
			"instance-id":       "eaab5544-76f6-4fa5-a7b3-6f19be51ecc1",
			"hostname":          "test",
			"facility":          "sjc1",
			"instance-type":     "baremetal_0",
			"public-ipv4":       "147.75.201.17",
			"public-ipv6":       "2604:1380:1000:e900::1",
			"packetMetaDataUrl": s.URL,
		},
	}.Test(t)
}
