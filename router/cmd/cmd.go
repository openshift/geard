package cmd

import (
	"encoding/json"
	"fmt"
	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/router"
	rjobs "github.com/openshift/geard/router/jobs"
	"github.com/openshift/geard/transport"
	"github.com/spf13/cobra"
	"os"
)

type Command struct {
	Transport *transport.TransportFlag
}

func (e *Command) RegisterRouterCmds(parent *cobra.Command) {
	routerCmd := &cobra.Command{
		Use:   "router",
		Short: "Commands to manage the router.",
		Run:   e.test,
	}
	parent.AddCommand(routerCmd)

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Test router extension.",
		Run:   e.test,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "create-frontend",
		Short: "Create a new frontend for the router to load balance.",
		Run:   e.createFrontend,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "add-alias",
		Short: "Add alias to a given frontend ([<host>/]<id> <alias>).",
		Run:   e.addAlias,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "delete-frontend",
		Short: "Delete an existing from the router.",
		Run:   e.removeFrontend,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "add-route",
		Short: "Add a new endpoint to an existing frontend for the router to load balance. Pass json argument e.g. : '{ \"Frontend\" : <frontendname>, \"Endpoints\" : [ {\"IP\" : <ipaddress>, \"Port\" : <port> } ] }' ",
		Run:   e.addRoute,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "delete-route",
		Short: "Remove an existing Endpoint from an existing frontend. Takes two arguments - [<frontendname> <endpoint-id>]. To get the endpoint-id to remove use print-routes <frontendname>",
		Run:   e.removeRoute,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "add-backend",
		Short: "Add a new endpoint to an existing frontend for the router to load balance.",
		Run:   addBackend,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "remove-backend",
		Short: "Remove an existing endpoint from an existing frontend of the router.",
		Run:   removeBackend,
	}
	routerCmd.AddCommand(testCmd)

	testCmd = &cobra.Command{
		Use:   "print-routes",
		Short: "Print an existing frontend's routes. Takes one argument - <frontendname>.",
		Run:   e.printFrontendRoutes,
	}
	routerCmd.AddCommand(testCmd)
}

func (e *Command) test(cmd *cobra.Command, args []string) {
	fmt.Println("Router testing.")
	frontendname := "geard-router-test"
	alias := "www.alias.com"
	router.CreateFrontend(frontendname, alias)

	// empty the global datastucture and re-read to confirm that correct output has been generated
	router.GlobalRoutes = nil
	router.ReadRoutes()
	frontend, ok := router.GlobalRoutes[frontendname]
	if !ok {
		fmt.Println("Test failed. Frontend created was not persisted.")
		return
	}
	if frontend.HostAliases[0] != alias {
		fmt.Println("Test failed. Frontend created did not persist its alias.")
		return
	}

	// create a route
	s := router.Endpoint{}
	s.IP = "localhost"
	s.Port = "4001"
	endpoints := make([]router.Endpoint, 1)
	endpoints[0] = s
	router.AddRoute(frontendname, "", "", nil, endpoints)

	// empty the db again and check for routes
	router.GlobalRoutes = nil
	router.ReadRoutes()
	frontend, ok = router.GlobalRoutes[frontendname]
	if !ok {
		fmt.Println("Test failed. Frontend created was not persisted.")
		return
	}
	if len(frontend.EndpointTable) != 1 {
		fmt.Println("Test failed. Created one route, but %d found.", len(frontend.EndpointTable))
		return
	}
	for _, ep := range frontend.EndpointTable {
		if ep.IP != "localhost" || ep.Port != "4001" {
			fmt.Println("Test failed. Created one route, but invalid values were persisted.", len(frontend.EndpointTable))
			return
		}
	}

	// good so far, now delete the testfrontend
	router.DeleteFrontend(frontendname)
	_, ok = router.GlobalRoutes[frontendname]
	if ok {
		fmt.Println("Test failed. Frontend deletion does not actually remove the frontend from the routing table.")
	}

	// all good
	fmt.Println(router.PrintRoutes())
}

func (e *Command) createFrontend(cmd *cobra.Command, args []string) {
	fmt.Println("Creating a new frontend")
	fmt.Println(args)
	if len(args) < 1 {
		fmt.Println("Atleast one argument expected for create-frontend (<frontendname> [<frontendurl>])")
		return
	}
	alias := ""
	if len(args) == 2 {
		alias = args[1]
	}

	t := e.Transport.Get()
	id, err := NewResourceLocator(t, "router", args[0])
	if err != nil {
		fmt.Println("frontendname should be either <host>/<name> or <name>. Where <host> is remote address of where the router resides")
		return
	}

	Executor{
		On: Locators{id},
		Serial: func(on Locator) JobRequest {
			return &rjobs.CreateFrontendRequest{
				Frontend: on.(*ResourceLocator).Id,
				Alias:    alias,
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func (e *Command) addAlias(cmd *cobra.Command, args []string) {
	fmt.Println("Adding alias to frontend")
	if len(args) != 2 {
		fmt.Println(args)
		fmt.Println("Two arguments needed for adding alias (<frontendname> <alias>).")
		return
	}
	t := e.Transport.Get()
	id, err := NewResourceLocator(t, "router", args[0])
	if err != nil {
		fmt.Println("frontendname should be either <host>/<name> or <name>. Where <host> is remote address of where the router resides.")
		return
	}

	Executor{
		On: Locators{id},
		Serial: func(on Locator) JobRequest {
			return &rjobs.AddAliasRequest{
				Frontend: on.(*ResourceLocator).Id,
				Alias:    args[1],
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func (e *Command) addRoute(cmd *cobra.Command, args []string) {
	fmt.Println("Adding route to frontend")
	fmt.Println(args)
	if len(args) != 1 {
		fmt.Println("Need one complex json argument to add route")
		return
	}
	var r rjobs.AddRouteRequest
	jerr := json.Unmarshal([]byte(args[0]), &r)
	if jerr != nil {
		fmt.Println("Error in unmarshalling input : %s", jerr.Error())
		return
	}
	if r.FrontendPath == "" {
		r.FrontendPath = "/"
	}
	if r.BackendPath == "" {
		r.BackendPath = "/"
	}

	t := e.Transport.Get()
	id, err := NewResourceLocator(t, "router", r.Frontend)
	if err != nil {
		fmt.Println("Json argument should have a valid 'Frontend' key. e.g.  { 'Frontend' : '<host>/<id>', Endpoints : [{ 'IP' : '<host_ip>', 'Port' : '<integer>'}] }")
		return
	}

	Executor{
		On: Locators{id},
		Serial: func(on Locator) JobRequest {
			return &rjobs.AddRouteRequest{
				Frontend:     on.(*ResourceLocator).Id,
				FrontendPath: r.FrontendPath,
				BackendPath:  r.BackendPath,
				Protocols:    r.Protocols,
				Endpoints:    r.Endpoints,
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()

	// router.AddRoute(r.Frontend, r.FrontendPath, r.BackendPath, r.Protocols, r.Endpoints)
}

func addBackend(cmd *cobra.Command, args []string) {
	fmt.Println("Adding a backend")
	fmt.Println(args)
	// TODO
}

func removeBackend(cmd *cobra.Command, args []string) {
	fmt.Println("Removing a backend")
	fmt.Println(args)
	// TODO
}

func (e *Command) removeFrontend(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println("One valid argument expected. (<host>/<id> | <id>)")
		return
	}
	t := e.Transport.Get()
	id, err := NewResourceLocator(t, "router", args[0])
	if err != nil {
		fmt.Println("One valid argument expected. (<host>/<id> | <id>)")
		return
	}

	Executor{
		On: Locators{id},
		Serial: func(on Locator) JobRequest {
			return &rjobs.DeleteFrontendRequest{
				Frontend: on.(*ResourceLocator).Id,
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func exists(s []string, e string) int {
	for i, a := range s {
		if a == e {
			return i
		}
	}
	return -1
}

func (e *Command) removeRoute(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println("Two string arguments to remove-route needed (<frontendname> <route-id>)")
		return
	}
	frontendname := args[0]
	epid := args[1]

	fmt.Printf("Deleting route %s from %s\n", epid, frontendname)
	delete(router.GlobalRoutes[frontendname].EndpointTable, epid)

	for _, be := range router.GlobalRoutes[frontendname].BeTable {
		i := exists(be.EndpointIds, epid)
		fmt.Printf("Deleting route %s from be_table, %d\n", epid, i)
		if i != -1 {
			// Bug here : https://code.google.com/p/go/issues/detail?id=3117
			// router.GlobalRoutes[frontendname].BeTable[be_id].EndpointIds = a
			//
			a := be.EndpointIds
			a[len(a)-1], a[i], a = "", a[len(a)-1], a[:len(a)-1]
			be.EndpointIds = a
			fmt.Printf("Deletion successful\n")
			break
		}
	}
	router.WriteRoutes()
	router.BumpRouter()
}

func (e *Command) printFrontendRoutes(cmd *cobra.Command, args []string) {
	frontendname := "*"
	if len(args) > 0 {
		frontendname = args[0]
	}
	t := e.Transport.Get()
	id, err := NewResourceLocator(t, "router", frontendname)
	if err != nil {
		fmt.Println("frontendname should be either <host>/<name> or <name>. Where <host> is remote address of where the router resides.")
		return
	}

	out, herr := Executor{
		On: Locators{id},
		Serial: func(on Locator) JobRequest {
			return &rjobs.GetRoutesRequest{
				Frontend: on.(*ResourceLocator).Id,
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.Gather()
	if herr != nil {
		fmt.Println(out)
	}
}
