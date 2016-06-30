package vm

import "testing"

func TestValidateMACWithValidMACs(t *testing.T) {
	validMACs := []string{
		"ba:47:78:65:e1:a5",
		"ea:4d:fa:a4:18:ec",
		"0e:54:fb:3a:45:d0",
		"4a:56:d3:0d:b6:86",
		"b6:8d:8f:7e:25:d9",
		"ae:21:ef:2a:ea:19",
		"a6:66:b0:f9:69:cc",
		"86:1c:55:d5:95:cc",
		"6a:f1:92:58:15:2b",
		"06:32:ba:a3:56:64",
		"1e:e7:e9:00:f2:20",
		"e6:c0:5d:a4:41:8c",
		"76:e6:ce:41:37:4f",
		"16:27:00:58:7c:18",
		"52:e2:96:70:a2:f0",
		"a6:75:88:79:e5:8c",
		"8e:7d:1c:f6:a3:a7",
		"5a:2e:8e:fa:12:d7",
		"02:3d:4b:b6:27:5e",
		"5e:f2:fc:19:55:cd",
		"76:7e:fc:a9:77:61",
		"8e:ba:28:10:88:f4",
		"4e:d1:96:54:2a:43",
		"e6:d8:46:69:ad:b1",
		"8e:e1:25:8b:e5:39",
		"0a:a7:2e:26:a7:91",
		"be:e5:91:b6:81:cf",
		"ba:d4:fc:b8:bb:fb",
		"ba:b4:5f:88:d5:29",
	}
	for _, mac := range validMACs {
		if err := validateMAC(mac); err != nil {
			t.Error("Unexpected error when validating: ", mac, " error: ", err)
		}
	}
}

func TestValidateMACWithGlobalMACs(t *testing.T) {
	// MAC assigned to organizations
	invalidMACs := []string{
		"00:0d:4b:ec:d6:18",
		"00:0d:4b:13:2e:61",
		"00:0d:4b:ca:85:1a",
		"00:0d:4b:aa:71:39",
		"00:0d:4b:2e:8f:cf",
		"00:0d:4b:b8:5a:c8",
		"00:0d:4b:0d:de:59",
		"00:0d:4b:ff:80:b6",
		"00:0d:4b:1e:58:5f",
		"00:0d:4b:65:e7:cf",
		"00:0d:4b:d6:74:6b",
		"00:00:0a:40:ce:1f",
		"00:00:0a:08:58:6c",
		"00:00:0a:e8:71:28",
		"00:00:0a:22:d9:44",
		"00:00:0a:fd:7c:18",
		"00:00:0a:0d:d6:ba",
		"00:00:0a:09:f2:01",
		"00:00:0a:5b:81:59",
	}
	for _, mac := range invalidMACs {
		if validateMAC(mac) == nil {
			t.Error("Expected error when validating: ", mac)
		}
	}
}

func TestValidateMACWithMulticastMACs(t *testing.T) {
	// Multicast MACs
	invalidMACs := []string{
		"df:db:a9:30:67:a1",
		"37:d6:97:2d:36:5f",
		"1f:0e:b2:1d:85:33",
		"f3:ba:9a:6b:75:d2",
		"9f:0a:11:bb:d5:63",
		"2f:92:df:14:1d:09",
		"0f:68:06:2c:e8:03",
		"0b:be:8a:5e:ca:d9",
		"df:f9:65:bf:b3:8b",
		"0f:28:1d:8c:3b:2d",
		"ab:5f:73:24:76:9e",
		"57:f6:92:22:ae:b4",
		"2b:3f:fd:22:51:fc",
		"5f:1d:64:0d:33:a7",
		"cf:25:60:e9:34:5f",
		"4b:13:de:0e:d0:5d",
		"af:75:7d:d6:31:56",
		"3f:38:a3:65:7c:e8",
		"93:66:dc:ba:27:21",
		"9b:8a:9e:db:ee:fd",
		"57:1f:74:bb:24:04",
		"a7:c4:a1:2f:0e:0e",
	}
	for _, mac := range invalidMACs {
		if validateMAC(mac) == nil {
			t.Error("Expected error when validating: ", mac)
		}
	}
}

func TestValidateMACWithInvalidMACs(t *testing.T) {
	// Malformated MACs
	invalidMACs := []string{
		"d:db:a9:30:67:a1",
		"37:d6:7:2d:36:5f",
		"1f:0e:b2:1d:8",
		"f3:ba:9a:6b:75:d2",
		"9f:0a:11:bbd5:63",
		"2f:92df:14:1d:09",
		"0f::06:2c:e8:03",
		"0b:be:tt:5e:ca:d9",
		"df:f9:65:qf:b3:8b",
		"0f:2l:1d:8c:3b:2d",
		"ab:5fd:73:24:76:9e",
		"57:f6a:92:22:ae:b4",
		"2b:3f:0fd:22:51:fc",
		"5f:1d:264:0d:33:a7",
		":47:78:65:e1:a5",
		"ea::fa:a4:18:ec",
		"0e:54w:fb:3a:45:d0",
		"4a56:d3:0d:b6:86",
		"b6:8d8f:7e:25:d9",
		"ae:21:ef2a:ea:19",
		"a6:66:b0:f9:69",
		"86:1c:55:-:95:cc",
		"6a:f1:?:58:15:2b",
		"1e:0xe7:e9:00:f2:20",
		"e6:c0:5d:%:41:8c",
		"",
		"--",
	}
	for _, mac := range invalidMACs {
		if validateMAC(mac) == nil {
			t.Error("Expected error when validating: ", mac)
		}
	}
}
