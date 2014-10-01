package main

import (
	"fmt"
	"os"
)

var (
	cmdEndpoints = &Command{
		Name:        "endpoints",
		Summary:     "Gets list of endpoints for a single service or summary for all endpoints",
		Usage:       "[service]",
		Description: "",
		Run:         runEndpoints,
	}
)

func init() {
}

func runEndpoints(args []string) (exit int) {

	if len(args) == 0 {
		services, err := containrunnerInstance.GetAllServices(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			return 1
		}

		for service_name, _ := range services {
			endpoints, err := containrunnerInstance.GetEndpointsForService(service_name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
				return 1
			}

			fmt.Printf("service %s has %d endpoints\n", service_name, len(endpoints))
		}

	}

	if len(args) == 1 {

		service_name := args[0]

		endpoints, err := containrunnerInstance.GetEndpointsForService(service_name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			return 1
		}

		for endpoint, endpointInfo := range endpoints {
			if endpointInfo != nil && endpointInfo.Revision != "" {
				fmt.Printf("service %s endpoint at %-22s running revision %s\n", service_name, endpoint, endpointInfo.Revision)
			} else {
				fmt.Printf("service %s endpoint at %s\n", service_name, endpoint)
			}
		}

		if len(endpoints) == 0 {
			fmt.Fprintf(os.Stderr, "No endpoints running for service %s!\n", service_name)
			return 1
		}

	}

	return 0
}
