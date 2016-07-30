package network

// Maximum time to wait for the xtables lock when using iptables
const xtableLockWait = "3"

// ipTableRules returns a list of commands to append rules for tapDevice.
// If delete=false, this returns the commands to delete the rules.
func ipTableRules(tapDevice string, ipPrefix string, delete bool) [][]string {
	subnet := ipPrefix + ".0/24"
	gateway := ipPrefix + ".1"
	prefixCommands := func(prefix []string, rules [][]string) [][]string {
		cmds := [][]string{}
		for _, rule := range rules {
			cmds = append(cmds, append(prefix, rule...))
		}
		return cmds
	}

	ruleAction := "-A"
	chainAction := "-N"
	if delete {
		ruleAction = "-D"
		chainAction = "-X"
	}

	// Create/delete custom chains for this tap device
	chains := prefixCommands([]string{"iptables", "-w", xtableLockWait, chainAction}, [][]string{
		{"input_" + tapDevice},
		{"output_" + tapDevice},
		{"fwd_input_" + tapDevice},
		{"fwd_output_" + tapDevice},
	})

	// Rules for jumping to custom chains for this tap device
	rules := prefixCommands([]string{"iptables", "-w", xtableLockWait, ruleAction}, [][]string{
		{"INPUT", "-i", tapDevice, "-j", "input_" + tapDevice},
		{"OUTPUT", "-o", tapDevice, "-j", "output_" + tapDevice},
		{"FORWARD", "-i", tapDevice, "-j", "fwd_input_" + tapDevice},
		{"FORWARD", "-o", tapDevice, "-j", "fwd_output_" + tapDevice},
	})

	// Rules for nat from this subnet
	nat := prefixCommands([]string{"iptables", "-w", xtableLockWait, "-t", "nat", ruleAction}, [][]string{
		{"POSTROUTING", "-o", "eth0", "-s", subnet, "-j", "MASQUERADE"},
	})

	// Rules for filtering INPUT from this tap device
	inputRules := prefixCommands([]string{"iptables", "-w", xtableLockWait, ruleAction, "input_" + tapDevice}, [][]string{
		// Allow requests to meta-data service (from subnet only)
		{"-p", "tcp", "-s", subnet, "-d", metaDataIP, "-m", "tcp", "--dport", "80", "-m", "state", "--state", "NEW,ESTABLISHED", "-j", "ACCEPT"},
		// Allow DNS requests
		{"-p", "tcp", "-s", subnet, "-d", gateway, "-m", "tcp", "--dport", "53", "-m", "state", "--state", "NEW,ESTABLISHED", "-j", "ACCEPT"},
		{"-p", "udp", "-s", subnet, "-d", gateway, "-m", "udp", "--dport", "53", "-m", "state", "--state", "NEW,ESTABLISHED", "-j", "ACCEPT"},
		// Allow DCHP requests
		{"-s", "0.0.0.0", "-d", "255.255.255.255", "-p", "udp", "-m", "udp", "--sport", "68", "--dport", "67", "-j", "ACCEPT"},
		{"-s", subnet, "-d", gateway, "-p", "udp", "-m", "udp", "--sport", "68", "--dport", "67", "-j", "ACCEPT"},
		// Reject all other input (with special case for wrong port on meta-data service)
		{"-s", subnet, "-d", metaDataIP, "-j", "REJECT", "--reject-with", "icmp-port-unreachable"},
		{"-j", "REJECT", "--reject-with", "icmp-host-unreachable"},
	})

	// Rules for filtering OUTPUT to this tap decice
	outputRules := prefixCommands([]string{"iptables", "-w", xtableLockWait, ruleAction, "output_" + tapDevice}, [][]string{
		// Allow meta-data replies (to subnet only)
		{"-p", "tcp", "-s", metaDataIP, "-d", subnet, "-m", "tcp", "--sport", "80", "-m", "state", "--state", "ESTABLISHED", "-j", "ACCEPT"},
		// Allow DNS replies from dnsmasq (to subnet only)
		{"-p", "udp", "-s", gateway, "-d", subnet, "-m", "udp", "--sport", "53", "-m", "state", "--state", "ESTABLISHED", "-j", "ACCEPT"},
		{"-p", "tcp", "-s", gateway, "-d", subnet, "-m", "tcp", "--sport", "53", "-m", "state", "--state", "ESTABLISHED", "-j", "ACCEPT"},
		// Allow DHCP replies
		{"-p", "udp", "-s", gateway, "-m", "udp", "--sport", "67", "--dport", "68", "-j", "ACCEPT"},
		// Reject all other output
		{"-j", "REJECT", "--reject-with", "icmp-net-prohibited"},
	})

	// Rules for filtering FORWARD from this tap device
	forwardInputRules := prefixCommands([]string{"iptables", "-w", xtableLockWait, ruleAction, "fwd_input_" + tapDevice}, [][]string{
		// Reject out-going from tctap1 to private subnets
		{"-d", "10.0.0.0/8", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		{"-d", "172.16.0.0/12", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		{"-d", "169.254.0.0/16", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		{"-d", "192.168.0.0/16", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		// Allow out-going from tctap1 with correct source subnet
		{"-o", "eth0", "-s", subnet, "-j", "ACCEPT"},
		// Allow tctap1 -> tctap1 withing allowed subnet
		{"-o", "tctap1", "-s", subnet, "-j", "ACCEPT"},
		// Reject all other input for forwarding from tap-device
		{"-j", "REJECT", "--reject-with", "icmp-net-prohibited"},
	})

	// Rules for filtering FORWARD to this tap device
	forwardOutputRules := prefixCommands([]string{"iptables", "-w", xtableLockWait, ruleAction, "fwd_output_" + tapDevice}, [][]string{
		// Reject incoming from private subnets to tctap1
		{"-s", "10.0.0.0/8", "-j", "DROP"},
		{"-s", "172.16.0.0/12", "-j", "DROP"},
		{"-s", "169.254.0.0/16", "-j", "DROP"},
		{"-s", "192.168.0.0/16", "-j", "DROP"},
		// Allow incoming from tctap1 with correct destination (if already established)
		{"-i", "eth0", "-d", subnet, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		// Allow tctap1 -> tctap1 withing allowed subnet
		{"-i", "tctap1", "-s", subnet, "-j", "ACCEPT"},
		// Reject all other output from forwarding to tap-device
		{"-j", "DROP"},
	})

	cmds := [][]string{}
	if !delete {
		cmds = append(cmds, nat...)
		cmds = append(cmds, chains...)
		cmds = append(cmds, rules...)
		cmds = append(cmds, inputRules...)
		cmds = append(cmds, outputRules...)
		cmds = append(cmds, forwardOutputRules...)
		cmds = append(cmds, forwardInputRules...)
	} else {
		// Reverse order when deleting, because we can't delete chains that are
		// referenced by a rule
		cmds = append(cmds, forwardInputRules...)
		cmds = append(cmds, forwardOutputRules...)
		cmds = append(cmds, outputRules...)
		cmds = append(cmds, inputRules...)
		cmds = append(cmds, rules...)
		cmds = append(cmds, chains...)
		cmds = append(cmds, nat...)
	}

	return cmds
}
