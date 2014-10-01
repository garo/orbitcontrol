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
		for endpoint, endpointInfo := range endpoints {
			if endpointInfo != nil && endpointInfo.Revision != "" {
				fmt.Printf("service %s endpoint at %-22s running revision %s\n", service, endpoint, endpointInfo.Revision)
			} else {
				fmt.Printf("service %s endpoint at %s\n", service, endpoint)
			}
		}

		if len(endpoints) == 0 {
			fmt.Fprintf(os.Stderr, "No endpoints running for service %s!\n", service)
			return 1
		}
	}

	return 0
}
