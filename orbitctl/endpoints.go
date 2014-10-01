package main

import (
	"fmt"
	"os"
)

var (
	cmdEndpoints = &Command{
		Name:        "endpoints",
		Summary:     "Gets list of endpoints for this service",
		Usage:       "<service>",
		Description: "",
		Run:         runEndpoints,
	}
)

func init() {
}

func runEndpoints(args []string) (exit int) {

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: orbitctl endpoints <service name>\n")
		return 1
	}

	service := args[0]

	endpoints, err := containrunnerInstance.GetEndpointsForService(service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		return 1
	} else {
		for _, endpoint := range endpoints {
			fmt.Printf("service %s endpoint at %s", service, endpoint)
		}
	}

	return 0
}
