package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
	"sort"
	"strings"
)

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "endpoints",
			Usage: "Gets list of endpoints for a single service or summary for all endpoints",
			Action: func(c *cli.Context) {
				fields := []string{}
				if c.String("fields") != "" {
					fields = strings.Split(c.String("fields"), ",")
				}
				runEndpoints(c.Args(), fields)
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "fields",
					Usage: "List of fields to print separated by comma. Usable for using output for scripting. Possibilites: [ip,endpoint,revision,port]",
				},
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

func runEndpoints(args []string, fields []string) (exit int) {

	if len(args) == 0 {
		services, err := containrunnerInstance.GetAllServices(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
			return 1
		}

		keys := []string{}
		for service_name, _ := range services {
			keys = append(keys, service_name)
		}

		sort.Strings(keys)

		for _, service_name := range keys {
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
			if len(fields) > 0 {
				var str = ""
				for _, field := range fields {
					if field == "ip" {
						str = fmt.Sprintf("%s%s ", str, endpoint[0:strings.Index(endpoint, ":")])
					} else if field == "port" {
						str = fmt.Sprintf("%s%s ", str, endpoint[strings.Index(endpoint, ":")+1:])
					} else if field == "endpoint" {
						str = fmt.Sprintf("%s%s ", str, endpoint)
					} else if field == "revision" {
						str = fmt.Sprintf("%s%s ", str, endpointInfo.Revision)
					}
				}
				if len(str) > 0 {
					str = str[0 : len(str)-1] // Strip tailing white space
					fmt.Printf("%s\n", str)
				}
			} else {
				if endpointInfo != nil && endpointInfo.Revision != "" {
					fmt.Printf("service %s endpoint at %-22s running revision %s\n", service_name, endpoint, endpointInfo.Revision)
				} else {
					fmt.Printf("service %s endpoint at %s\n", service_name, endpoint)
				}
			}
		}

		if len(endpoints) == 0 {
			if len(fields) == 0 {
				fmt.Fprintf(os.Stderr, "No endpoints running for service %s!\n", service_name)
			}
			return 1
		}

	}

	return 0
}
