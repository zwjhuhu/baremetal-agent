package main

import (
	"baremetal/plugin"
	"baremetal/server"
	"baremetal/utils"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

const (
	VIRTIO_PORT_PATH     = "/dev/virtio-ports/applianceVm.port"
	AGENT_CONFIG_FILE    = "/var/lib/uit/baremetal/agent.conf"
	TMP_LOCATION_FOR_ESX = "/tmp/bootstrap-info.json"
	// use this rule number to set a rule which confirm route entry work issue ZSTAC-6170
	ROUTE_STATE_NEW_ENABLE_FIREWALL_RULE_NUMBER = 9999
)

var bootstrapInfo map[string]interface{} = make(map[string]interface{})

func loadPlugins() {
	plugin.ApvmEntryPoint()
	// plugin.DhcpEntryPoint()
	// plugin.MiscEntryPoint()
	// plugin.DnsEntryPoint()
	// plugin.SnatEntryPoint()
	// plugin.DnatEntryPoint()
	// plugin.VipEntryPoint()
	// plugin.EipEntryPoint()
	// plugin.LbEntryPoint()
	// plugin.IPsecEntryPoint()
	// plugin.ConfigureNicEntryPoint()
	// plugin.RouteEntryPoint()
	// plugin.ZsnEntryPoint()
	// plugin.PrometheusEntryPoint()
	// plugin.OspfEntryPoint()
}

var options server.Options

func abortOnWrongOption(msg string) {
	fmt.Println(msg)
	flag.Usage()
	os.Exit(1)
}

func parseAgentConfigInfo() {

	content, err := ioutil.ReadFile(AGENT_CONFIG_FILE)
	utils.PanicOnError(err)
	if err = json.Unmarshal(content, &bootstrapInfo); err != nil {
		panic(errors.Wrap(err, fmt.Sprintf("unable to JSON parse:\n %s", string(content))))
	}
	checkAgentConfigInfo()
}

func findPxenicIps(nic string) ([]string, error) {
	bash := utils.Bash{
		Command: fmt.Sprintf("ip -o -f inet addr show %s | awk '/scope global/ {print $4}'", nic),
	}
	ret, o, _, err := bash.RunWithReturn()
	var ips []string
	if err != nil {
		return ips, err
	}
	if ret != 0 {
		return ips, errors.New(fmt.Sprintf("no ip with the nic[%s] found in the system", nic))
	}

	o = strings.TrimSpace(o)
	ips = strings.Split(o, "\n")
	return ips, nil
}

func checkAgentConfigInfo() {
	pxenic := ""
	tmp, ok := bootstrapInfo["pxenic"]
	if !ok {
		panic(errors.New("pxenic config error"))
	} else {
		pxenic = tmp.(string)
	}
	b := utils.Bash{
		Command: fmt.Sprintf("ip link show dev %s &>/dev/null", pxenic),
	}
	b.Run()
	b.PanicIfError()

	ips, err := findPxenicIps(pxenic)
	if err != nil {
		panic(err)
	} else if len(ips) == 0 {
		panic(errors.New(fmt.Sprintf("no ip find from nic %s", pxenic)))
	}

	dhcpStartIp := ""
	tmp, ok = bootstrapInfo["dhcpStartIp"]
	if ok {
		dhcpStartIp = tmp.(string)
	}
	dhcpEndIp := ""
	tmp, ok = bootstrapInfo["dhcpEndIp"]
	if ok {
		dhcpEndIp = tmp.(string)
	}
	ip1 := ""
	ip2 := ""

	if dhcpStartIp != "" {
		for _, cidr := range ips {
			if utils.CheckCIDRContainsIp(dhcpStartIp, cidr) {
				os := strings.Split(cidr, "/")
				ip1 = os[0]
				break
			}
		}
	}
	if dhcpEndIp != "" {
		for _, cidr := range ips {
			if utils.CheckCIDRContainsIp(dhcpEndIp, cidr) {
				os := strings.Split(cidr, "/")
				ip2 = os[0]
				break
			}
		}
	}

	pxeip := ip1
	if ip1 == "" && ip2 == "" {
		// 使用第一个ip
		os := strings.Split(ips[0], "/")
		pxeip = os[0]
	} else if ip1 != "" && ip2 == "" {
		pxeip = ip1
	} else if ip2 != "" && ip1 == "" {
		pxeip = ip2
	} else if ip1 != ip2 {
		// startIP 和 endIP 不在一个网络段
		panic(errors.New(fmt.Sprintf("dhcp startIP %s and endIP %s not in same cidr", ip1, ip2)))
	}
	log.Debugf("choose pxe ip %s", pxeip)

	// config dnsmasq

}

func parseCommandOptions() {
	options = server.Options{}
	flag.StringVar(&options.Ip, "ip", "", "The IP address the server listens on")
	flag.UintVar(&options.Port, "port", 7272, "The port the server listens on")
	flag.UintVar(&options.ReadTimeout, "readtimeout", 10, "The socket read timeout")
	flag.UintVar(&options.WriteTimeout, "writetimeout", 10, "The socket write timeout")
	flag.StringVar(&options.LogFile, "logfile", "zvr.log", "The log file path")

	flag.Parse()

	if options.Ip == "" {
		abortOnWrongOption("error: the options 'ip' is required")
	}

	server.SetOptions(options)
}

func configureZvrFirewall() {
	if utils.IsSkipVyosIptables() {
		err := utils.InitNicFirewall("eth0", options.Ip, true, utils.ACCEPT)
		if err != nil {
			log.Debugf("zvr configureZvrFirewall failed %s", err.Error())
		}
		return
	}

	tree := server.NewParserFromShowConfiguration().Tree

	/* add description to avoid duplicated firewall rule when reconnect vr */
	des := "management-port-rule"
	if r := tree.FindFirewallRuleByDescription("eth0", "local", des); r != nil {
		r.Delete()
	}

	tree.SetFirewallOnInterface("eth0", "local",
		fmt.Sprintf("destination address %v", options.Ip),
		fmt.Sprintf("destination port %v", options.Port),
		"protocol tcp",
		"action accept",
		fmt.Sprintf("description %s", des),
	)

	tree.Apply(false)
}

func main() {

	utils.InitLog("/var/lib/uit/baremetal-agent.log", false)
	parseAgentConfigInfo()

	loadPlugins()
	// server.VyosLockInterface(configureZvrFirewall)()
	options := server.Options{
		Ip:           "0.0.0.0",
		Port:         10002,
		ReadTimeout:  10,
		WriteTimeout: 10,
	}
	server.SetOptions(options)
	server.Start()
}
