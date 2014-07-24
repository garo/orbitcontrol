package main

import (
	"fmt"
	"os"
)

var (
	cmdServices = &Command{
		Name:        "services",
		Summary:     "List known services",
		Usage:       "",
		Description: "",
		Run:         runServices,
	}
)

func init() {
}

func runServices(args []string) (exit int) {

	if len(args) == 0 {
		// list services
		services, err := containrunnerInstance.GetAllServices(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting services: %+v\n", err)
			return 1
		}
		fmt.Printf("%+v\n", services)

		fmt.Printf("Found %d services\n", len(services))
		for name, service := range services {
			fmt.Printf("Name: %s, EndpointPort: %d\n", name, service.EndpointPort)
		}
		return 0
	}

	switch {
	case args[0] == "del" || args[0] == "delete":
		if len(args) > 0 {
			for _, service := range args[1:] {
				err := containrunnerInstance.RemoveService(service, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not remove service %s. Reason: %+v\n", service, err)
				} else {
					fmt.Printf("Removing service %s\n", service)
				}
			}

		}

	}

	return 0
}
