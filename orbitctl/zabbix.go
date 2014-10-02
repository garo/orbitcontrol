package main

import (
	"fmt"
	"os"
)

var (
	cmdZabbix = &Command{
		Name:    "zabbix",
		Summary: "Outputs data to be sent to zabbix. Multiple different modes.",
		Usage:   "[mode, for example endpoints]",
		Description: `Modes:
endpoints: Outputs number of endpoints. Uses key orbit.running_endpoints with service name as the host
`,
		Run: runZabbix,
	}
)

func init() {
}

func runZabbix(args []string) (exit int) {

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Error, see usage\n")
		return 1
	}

	if args[0] == "endpoints" {
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

			fmt.Printf("%s orbit.running_endpoints %d\n", service_name, len(endpoints))
		}

	}

	return 0
}
