package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
)

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "zabbix",
			Usage: "Outputs data to be sent to zabbix. Multiple different modes.",
			Subcommands: []cli.Command{
				{
					Name:  "endpoints",
					Usage: "Outputs number of endpoints. Uses key orbit.running_endpoints with service name as the host",
					Action: func(c *cli.Context) {
						services, err := containrunnerInstance.GetAllServices(nil)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
							os.Exit(1)
						}

						for service_name, _ := range services {
							endpoints, err := containrunnerInstance.GetEndpointsForService(service_name)
							if err != nil {
								fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
								os.Exit(1)
							}

							fmt.Printf("%s orbit.running_endpoints %d\n", service_name, len(endpoints))
						}
					},
				},
			},
		})
}
