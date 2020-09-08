package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

const (
	VROUTER_ROUTE_PROTO = "vrouter"
	//ZSTACK_ROUTE_PROTO_IDENTIFFER = "192" conflict with eigrp
	VROUTER_ROUTE_PROTO_IDENTIFFER = "199"
)

func NetmaskToCIDR(netmask string) (int, error) {
	countBit := func(num uint) int {
		count := uint(0)
		var i uint
		for i = 31; i > 0; i-- {
			count += ((num << i) >> uint(31)) & uint(1)
		}

		return int(count)
	}

	cidr := 0
	for _, o := range strings.Split(netmask, ".") {
		num, err := strconv.ParseUint(o, 10, 32)
		if err != nil {
			return -1, err
		}
		cidr += countBit(uint(num))
	}

	return cidr, nil
}

func GetNetworkNumber(ip, netmask string) (string, error) {
	ips := strings.Split(ip, ".")
	masks := strings.Split(netmask, ".")

	ipInByte := make([]interface{}, 4)
	for i := 0; i < len(ips); i++ {
		p, err := strconv.ParseUint(ips[i], 10, 32)
		if err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("unable to get network number[ip:%v, netmask:%v]", ip, netmask))
		}
		m, err := strconv.ParseUint(masks[i], 10, 32)
		PanicOnError(err)
		if err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("unable to get network number[ip:%v, netmask:%v]", ip, netmask))
		}
		ipInByte[i] = p & m
	}

	cidr, err := NetmaskToCIDR(netmask)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to get network number[ip:%v, netmask:%v]", ip, netmask))
	}

	return fmt.Sprintf("%v.%v.%v.%v/%v", ipInByte[0], ipInByte[1], ipInByte[2], ipInByte[3], cidr), nil
}

type Nic struct {
	Name string
	Mac  string
}

func (nic Nic) String() string {
	s, _ := json.Marshal(nic)
	return string(s)
}

func GetAllNics() (map[string]Nic, error) {
	const ROOT = "/sys/class/net"

	files, err := ioutil.ReadDir(ROOT)
	if err != nil {
		return nil, err
	}

	nics := make(map[string]Nic)
	for _, f := range files {
		if f.IsDir() || f.Name() == "lo" || strings.Contains(f.Name(), "ifb") {
			continue
		}

		macfile := filepath.Join(ROOT, f.Name(), "address")
		mac, err := ioutil.ReadFile(macfile)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("unable to read the mac file[%s]", macfile))
		}
		nics[f.Name()] = Nic{
			Name: strings.TrimSpace(f.Name()),
			Mac:  strings.TrimSpace(string(mac)),
		}
	}

	return nics, nil
}

func GetNicNameByMac(mac string) (string, error) {
	nics, err := GetAllNics()
	if err != nil {
		return "", err
	}

	for _, nic := range nics {
		if nic.Mac == mac {
			/* for vlan sub interface, nic.name is eth1.100 */
			name := strings.Split(nic.Name, ".")
			return name[0], nil
		}
	}

	return "", fmt.Errorf("cannot find any nic with the mac[%s]", mac)
}

func GetNicNameByIp(ip string) (string, error) {
	bash := Bash{
		Command: fmt.Sprintf("ip addr | grep -w %s", ip),
	}
	ret, o, _, err := bash.RunWithReturn()
	if err != nil {
		return "", err
	}
	if ret != 0 {
		return "", errors.New(fmt.Sprintf("no nic with the IP[%s] found in the system", ip))
	}

	o = strings.TrimSpace(o)
	os := strings.Split(o, " ")
	return os[len(os)-1], nil
}

func GetIpByNicName(nic string) (string, error) {
	bash := Bash{
		Command: fmt.Sprintf("ip -o -f inet addr show %s | awk '/scope global/ {print $4}'", nic),
	}
	ret, o, _, err := bash.RunWithReturn()
	if err != nil {
		return "", err
	}
	if ret != 0 {
		return "", errors.New(fmt.Sprintf("no ip with the nic[%s] found in the system", nic))
	}

	o = strings.TrimSpace(o)
	os := strings.Split(o, "/")
	return os[0], nil
}

func GetIpFromUrl(url string) (string, error) {
	ip := strings.Split(strings.Split(url, "/")[2], ":")[0]
	return ip, nil
}

func CheckVrouterRouteExists(ip string) bool {
	bash := Bash{
		Command: fmt.Sprintf("ip r list %s/32 proto %s", ip, VROUTER_ROUTE_PROTO),
	}
	_, o, _, _ := bash.RunWithReturn()
	if o == "" {
		return false
	}
	return true
}

func DeleteRouteIfExists(ip string) error {
	if CheckVrouterRouteExists(ip) == true {
		bash := Bash{
			Command: fmt.Sprintf("ip route del %s/32", ip),
		}
		_, _, _, err := bash.RunWithReturn()
		if err != nil {
			return err
		}
	}

	return nil
}

func SetVrouterRoute(ip string, nic string, gw string) error {
	SetVrouterRouteProtoIdentifier()
	DeleteRouteIfExists(ip)

	var bash Bash
	if gw == "" {
		bash = Bash{
			Command: fmt.Sprintf("ip route add %s/32 dev %s proto %s", ip, nic, VROUTER_ROUTE_PROTO),
		}
	} else {
		bash = Bash{
			Command: fmt.Sprintf("ip route add %s/32 via %s dev %s proto %s", ip, gw, nic, VROUTER_ROUTE_PROTO),
		}
	}

	ret, _, _, err := bash.RunWithReturn()
	if err != nil {
		return err
	}
	// NOTE(WeiW): It will return 2 if exists
	if ret != 0 && ret != 2 {
		return errors.New(fmt.Sprintf("add route to %s/32 via %s dev %s failed", ip, gw, nic))
	}

	return nil
}

func GetNicForRoute(ip string) string {
	bash := Bash{
		Command: fmt.Sprintf("ip -o r get %s | awk '{print $3}'", ip),
	}
	_, o, _, err := bash.RunWithReturn()
	PanicOnError(err)
	o = strings.Replace(o, "\n", "", -1)
	return o
}

func RemoveVrouterRoute(ip string) error {
	SetVrouterRouteProtoIdentifier()
	bash := Bash{
		Command: fmt.Sprintf("ip route del %s/32 proto %s", ip, VROUTER_ROUTE_PROTO),
	}
	ret, _, _, err := bash.RunWithReturn()
	if err != nil {
		return err
	}
	if ret != 0 {
		return errors.New(fmt.Sprintf("del route to %s/32 proto %s failed", ip, VROUTER_ROUTE_PROTO))
	}

	return nil
}

func SetVrouterRouteProtoIdentifier() {
	bash := Bash{
		Command: "grep vrouter /etc/iproute2/rt_protos",
	}
	check, _, _, _ := bash.RunWithReturn()

	if check != 0 {
		log.Debugf("no route proto vrouter in /etc/iproute2/rt_protos")
		bash = Bash{
			Command: fmt.Sprintf("sudo bash -c \"echo -e '\n\n# Used by vrouter\n%s     vrouter' >> /etc/iproute2/rt_protos\"", VROUTER_ROUTE_PROTO_IDENTIFFER),
		}
		bash.Run()
	}
}

func GetNicNumber(nic string) (int, error) {
	num, err := strconv.ParseInt(strings.Split(nic, "eth")[1], 10, 64)
	if err != nil {
		return -1, err
	}
	return int(num), nil
}

func CheckCIDRContainsIp(ip string, cidr string) bool {
	if ip == "" || cidr == "" {
		return false
	}
	_, subnet, err := net.ParseCIDR(cidr)
	PanicOnError(err)
	return subnet.Contains(net.ParseIP(ip))
}

func GetPrivteInterface() []string {
	bash := Bash{
		Command: fmt.Sprintf("ip link | grep -B 2 'category:Private' | grep '<BROADCAST,MULTICAST' | awk -F ':' '{print $2}'"),
	}
	ret, o, _, err := bash.RunWithReturn()
	if err != nil {
		return nil
	}
	if ret != 0 {
		return nil
	}

	lines := strings.Split(o, "\n")
	var nics []string
	for _, name := range lines {
		name = strings.Trim(name, " ")
		if name != "" {
			nics = append(nics, name)
		}
	}

	if len(nics) == 0 {
		return nil
	}

	return nics
}

func CleanConnTrackConnection(ip string, proto string, port int) error {
	var command string
	if proto == "" {
		command = fmt.Sprintf("sudo conntrack -d %s -D", ip)
	} else if port == 0 {
		command = fmt.Sprintf("sudo conntrack -d %s -p %s -D", ip, proto)
	} else {
		command = fmt.Sprintf("sudo conntrack -d %s -p %s --dport %d -D", ip, proto, port)
	}

	bash := Bash{
		Command: command,
	}
	ret, _, _, err := bash.RunWithReturn()
	if err != nil {
		return err
	}
	if ret != 0 {
		return errors.New(fmt.Sprintf("conntrack -d %s -p %s --dport %d -D failed return %d", ip, proto, port, ret))
	}

	return nil
}
