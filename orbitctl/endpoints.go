package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
)

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "endpoints",
			Usage: "Gets list of endpoints for a single service or summary for all endpoints",
			Action: func(c *cli.Context) {
				runEndpoints(c.Args())
			},
			BashComplete: func(c *cli.Context) {
				services, err := containrunnerInstance.GetAllServices(nil)
				if err != nil {
					fmt.Printf("Error: %+v\n", err)
					return
				}
				if len(c.Args()) > 0 {
					fmt.Printf("BASH_COMPETITON_MYSTERY %+v\n", c.Args())
					return
				}

				for service, _ := range services {
					fmt.Println(service)
				}
			},
		})
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
