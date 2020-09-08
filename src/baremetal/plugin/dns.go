package plugin

import (
	"fmt"
	"zvr/server"
	"zvr/utils"
)

const (
	REMOVE_DNS_PATH = "/removedns"
	SET_DNS_PATH    = "/setdns"
	SET_VPNDNS_PATH = "/setvpcdns"
)

type dnsInfo struct {
	DnsAddress string `json:"dnsAddress"`
	NicMac     string `json:"nicMac"`
}

type setDnsCmd struct {
	//Dns []dnsInfo `json:"dns"`
	Dns    []string `json:"dns"`
	NicMac []string `json:"nicMac"`
}

type removeDnsCmd struct {
	//Dns []dnsInfo `json:"dns"`
	Dns    []string `json:"dns"`
	NicMac []string `json:"nicMac"`
}

type setVpcDnsCmd struct {
	Dns    []string `json:"dns"`
	NicMac []string `json:"nicMac"`
}

func makeDnsFirewallRuleDescription(nicname string) string {
	return fmt.Sprintf("DNS-for-%s", nicname)
}

func setDnsFirewallRules(nicName string) error {
	rule := utils.NewIptablesRule(utils.UDP, "", "", 0, 53, nil, utils.RETURN, utils.DnsRuleComment)
	utils.InsertFireWallRule(nicName, rule, utils.LOCAL)
	rule = utils.NewIptablesRule(utils.TCP, "", "", 0, 53, nil, utils.RETURN, utils.DnsRuleComment)
	utils.InsertFireWallRule(nicName, rule, utils.LOCAL)
	return nil
}

func removeDnsFirewallRules(nicName string) error {
	utils.DeleteLocalFirewallRuleByComment(nicName, utils.DnsRuleComment)
	return nil
}

func setDnsHandler(ctx *server.CommandContext) interface{} {
	tree := server.NewParserFromShowConfiguration().Tree

	cmd := &setDnsCmd{}
	ctx.GetCommand(cmd)

	/* delete previous config */
	tree.Deletef("service dns forwarding")

	tree.Setf("service dns forwarding allow-from 0.0.0.0/0")
	/* dns is ordered in management node, should not be changed in vyos */
	for _, dns := range cmd.Dns {
		tree.SetfWithoutCheckExisting("service dns forwarding name-server %s", dns)
	}

	for _, mac := range cmd.NicMac {
		eth, err := utils.GetNicNameByMac(mac)
		utils.PanicOnError(err)
		ip, err := utils.GetIpByNicName(eth)
		utils.PanicOnError(err)

		// tree.SetfWithoutCheckExisting("service dns forwarding listen-on %s", eth)

		tree.SetfWithoutCheckExisting("service dns forwarding listen-address %s", ip)
		if utils.IsSkipVyosIptables() {
			setDnsFirewallRules(eth)
		} else {
			des := makeDnsFirewallRuleDescription(eth)
			if r := tree.FindFirewallRuleByDescription(eth, "local", des); r == nil {
				tree.SetFirewallOnInterface(eth, "local",
					fmt.Sprintf("description %v", des),
					"destination port 53",
					"protocol tcp_udp",
					"action accept",
				)

				tree.AttachFirewallToInterface(eth, "local")
			}
		}
	}

	tree.Apply(false)

	return nil
}

func removeDnsHandler(ctx *server.CommandContext) interface{} {
	tree := server.NewParserFromShowConfiguration().Tree

	cmd := &removeDnsCmd{}
	ctx.GetCommand(cmd)

	for _, dns := range cmd.Dns {
		tree.Deletef("service dns forwarding name-server %s", dns)
	}

	for _, mac := range cmd.NicMac {
		eth, err := utils.GetNicNameByMac(mac)
		utils.PanicOnError(err)
		ip, err := utils.GetIpByNicName(eth)
		utils.PanicOnError(err)

		if ip != "" {
			tree.Deletef("service dns forwarding listen-address %s", ip)
		}
	}

	tree.Apply(false)
	return nil
}

func setVpcDnsHandler(ctx *server.CommandContext) interface{} {
	tree := server.NewParserFromShowConfiguration().Tree

	cmd := &setVpcDnsCmd{}
	ctx.GetCommand(cmd)

	/* remove old dns  */
	tree.Deletef("service dns")
	priNics := utils.GetPrivteInterface()
	for _, priNic := range priNics {
		if utils.IsSkipVyosIptables() {
			removeDnsFirewallRules(priNic)
		} else {
			des := makeDnsFirewallRuleDescription(priNic)
			if r := tree.FindFirewallRuleByDescription(priNic, "local", des); r != nil {
				r.Delete()
			}
		}
	}

	if len(cmd.Dns) == 0 || len(cmd.NicMac) == 0 {
		tree.Apply(false)
		return nil
	}

	/* add new configure */
	var nics []string
	for _, mac := range cmd.NicMac {
		eth, err := utils.GetNicNameByMac(mac)
		utils.PanicOnError(err)
		nics = append(nics, eth)
	}
	if len(nics) == 0 {
		tree.Apply(false)
		return nil
	}

	for _, nic := range nics {
		//tree.SetfWithoutCheckExisting("service dns forwarding listen-on %s", nic)

		ip, err := utils.GetIpByNicName(nic)
		utils.PanicOnError(err)

		tree.SetfWithoutCheckExisting("service dns forwarding listen-address %s", ip)

		if utils.IsSkipVyosIptables() {
			setDnsFirewallRules(nic)
		} else {
			des := makeDnsFirewallRuleDescription(nic)
			if r := tree.FindFirewallRuleByDescription(nic, "local", des); r == nil {
				tree.SetFirewallOnInterface(nic, "local",
					fmt.Sprintf("description %v", des),
					"destination port 53",
					"protocol tcp_udp",
					"action accept",
				)

				tree.AttachFirewallToInterface(nic, "local")
			}
		}
	}

	for _, dns := range cmd.Dns {
		tree.SetfWithoutCheckExisting("service dns forwarding name-server %s", dns)
	}

	tree.Apply(false)
	return nil
}

func DnsEntryPoint() {
	server.RegisterAsyncCommandHandler(SET_DNS_PATH, server.VyosLock(setDnsHandler))
	server.RegisterAsyncCommandHandler(REMOVE_DNS_PATH, server.VyosLock(removeDnsHandler))
	server.RegisterAsyncCommandHandler(SET_VPNDNS_PATH, server.VyosLock(setVpcDnsHandler))
}
