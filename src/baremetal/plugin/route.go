package plugin

import (
	"fmt"
	"strings"
	"zvr/server"
	"zvr/utils"

	"github.com/pkg/errors"
)

const (
	ADD_ROUTES    = "/addroutes"
	REMOVE_ROUTES = "/removeroutes"
	SYNC_ROUTES   = "/syncroutes"
	GET_ROUTES    = "/getroutes"
)

type routeInfo struct {
	Destination string `json:"destination"`
	Target      string `json:"target"`
	Distance    int    `json:"distance"`
}

type SyncRoutesCmd struct {
	Routes []routeInfo `json:"routes"`
}

type GetRoutesRsp struct {
	RawRoutes string `json:"rawRoutes"`
}

func syncRoutes(ctx *server.CommandContext) interface{} {
	cmd := &SyncRoutesCmd{}
	ctx.GetCommand(cmd)

	setRoutes(cmd.Routes)
	return nil
}

func addRoutes(ctx *server.CommandContext) interface{} {
	cmd := &SyncRoutesCmd{}
	ctx.GetCommand(cmd)

	doAddRoutes(cmd.Routes)
	return nil
}

func removeRoutes(ctx *server.CommandContext) interface{} {
	cmd := &SyncRoutesCmd{}
	ctx.GetCommand(cmd)

	doRemoveRoutes(cmd.Routes)
	return nil
}

func setRoutes(infos []routeInfo) {
	tree := server.NewParserFromShowConfiguration().Tree
	if rs := tree.Get("protocols static route"); rs != nil {
		for _, r := range rs.Children() {
			if !strings.Contains(r.String(), "0.0.0.0") {
				r.Delete()
			}
		}
	}

	for _, route := range infos {
		if route.Destination != "" && !strings.Contains(route.Destination, "0.0.0.0") {
			if route.Target == "" {
				tree.Setf("protocols static route %s blackhole distance %d", route.Destination, route.Distance)
			} else {
				tree.Setf("protocols static route %s next-hop %s distance %d", route.Destination, route.Target, route.Distance)
			}
		}
	}

	tree.Apply(false)
}

func doAddRoutes(infos []routeInfo) {
	tree := server.NewParserFromShowConfiguration().Tree

	for _, route := range infos {

		if route.Destination != "" && !strings.Contains(route.Destination, "0.0.0.0") {
			if route.Target == "" {
				part := "protocols static route %s blackhole distance %d"
				if tree.Has(fmt.Sprintf(part, route.Destination, route.Distance)) {
					tree.Deletef(part, route.Destination, route.Distance)
				}
				tree.Setf(part, route.Destination, route.Distance)
			} else {
				part := "protocols static route %s next-hop %s distance %d"
				if tree.Has(fmt.Sprintf(part, route.Destination, route.Target, route.Distance)) {
					tree.Deletef(part, route.Destination, route.Target, route.Distance)
				}
				tree.Setf(part, route.Destination, route.Target, route.Distance)
			}
		}

	}

	tree.Apply(false)
}

func doRemoveRoutes(infos []routeInfo) {
	tree := server.NewParserFromShowConfiguration().Tree

	for _, route := range infos {
		if route.Destination != "" && !strings.Contains(route.Destination, "0.0.0.0") {
			if route.Target == "" {
				tree.Deletef("protocols static route %s blackhole distance %d", route.Destination, route.Distance)
			} else {
				tree.Deletef("protocols static route %s next-hop %s distance %d", route.Destination, route.Target, route.Distance)
			}
		}
	}

	tree.Apply(false)
}

func getRoutes(ctx *server.CommandContext) interface{} {
	// Note(WeiW): add "vtysh -c "show ip route " >/dev/null" to get correct return code
	bash := utils.Bash{
		Command: fmt.Sprintf("vtysh -c 'show ip route' | tail -n +7; vtysh -c 'show ip route' >/dev/null"),
	}
	ret, o, _, err := bash.RunWithReturn()
	utils.PanicOnError(err)
	if ret != 0 {
		utils.PanicOnError(errors.Errorf(("get route from zebra error")))
	}
	return GetRoutesRsp{RawRoutes: o}
}

func RouteEntryPoint() {
	server.RegisterAsyncCommandHandler(ADD_ROUTES, server.VyosLock(addRoutes))
	server.RegisterAsyncCommandHandler(REMOVE_ROUTES, server.VyosLock(removeRoutes))
	server.RegisterAsyncCommandHandler(SYNC_ROUTES, server.VyosLock(syncRoutes))
	server.RegisterSyncCommandHandler(GET_ROUTES, getRoutes)
}
