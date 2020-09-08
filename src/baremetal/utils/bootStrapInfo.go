package utils

import (
	"encoding/json"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
)

const (
	BOOTSTRAP_INFO_CACHE = "/home/vyos/zvr/bootstrap-info.json"
	DEFAULT_SSH_PORT     = 22
)

var bootstrapInfo map[string]interface{} = make(map[string]interface{})

func GetSshPortFromBootInfo() float64 {
	port, ok := bootstrapInfo["sshPort"].(float64)
	if !ok {
		return DEFAULT_SSH_PORT
	}

	return port
}

func GetMgmtInfoFromBootInfo() map[string]interface{} {
	mgmtNic := bootstrapInfo["managementNic"].(map[string]interface{})
	return mgmtNic
}

func IsSkipVyosIptables() bool {
	SkipVyosIptables, ok := bootstrapInfo["skipVyosIptables"].(bool)
	if !ok {
		return false
	}

	return SkipVyosIptables
}

// 增加一个routerid表示与路由虚机的id
func GetRouterid() string {
	routerid, ok := bootstrapInfo["routerid"].(string)
	if !ok {
		return ""
	}

	return routerid
}

func InitBootStrapInfo() {
	content, err := ioutil.ReadFile(BOOTSTRAP_INFO_CACHE)
	PanicOnError(err)
	if len(content) == 0 {
		log.Debugf("no content in %s, can not get mgmt gateway", BOOTSTRAP_INFO_CACHE)
	}

	if err := json.Unmarshal(content, &bootstrapInfo); err != nil {
		log.Debugf("can not parse info from %s, can not get mgmt gateway", BOOTSTRAP_INFO_CACHE)
	}

	log.Debugf("skipVyosIptables %t", bootstrapInfo["skipVyosIptables"].(bool))
}
