package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/garo/orbitcontrol/containrunner"

	"time"
)

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "queue",
			Usage: "Queue testing function.",
			Subcommands: []cli.Command{
				{
					Name:  "testpublish",
					Usage: "",
					Action: func(c *cli.Context) {

						events := containrunnerInstance.Events
						err := events.Init("amqp://guest:guest@localhost:5672/")
						if err != nil {
							fmt.Printf("Error connecting to broker: %+v\n", err)
							return
						}

						if c.Args().First() != "" {
							for {
								fmt.Printf("publish string to queue: '%s'\n", c.Args().First())

								e := containrunner.DeploymentEvent{
									"service name",
									"user name",
									"revision id",
								}

								fmt.Printf("Publishing to mq\n")
								err := events.PublishDeploymentEvent(time.Now(), e)
								if err != nil {
									fmt.Printf("Error on publish: %+v\n", err)
								}

								time.Sleep(5 * time.Second)
							}
						}
					},
				},

				{
					Name:  "listen",
					Usage: "",
					Action: func(c *cli.Context) {

						events := containrunnerInstance.Events
						err := events.Init("amqp://guest:guest@localhost:5672/")
						if err != nil {
							fmt.Printf("Error connecting to broker: %+v\n", err)
							return
						}
						fmt.Printf("Listening for events...")
						for d := range events.GetReceiveredEventChannel() {
							fmt.Printf("Got deployment event: %+v\n", d)
						}
					},
				},
			},
		})
}
