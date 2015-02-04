package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/garo/orbitcontrol/containrunner"
)

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "dblog",
			Usage: "Starts message broker to database logger instance",
			Action: func(c *cli.Context) {
				dburi := c.String("dburi")
				if dburi == "" {
					cli.ShowSubcommandHelp(c)
					return
				}

				etcdClient := containrunner.GetEtcdClient(containrunnerInstance.EtcdEndpoints)

				globalConfiguration, err := containrunnerInstance.GetGlobalOrbitProperties(etcdClient)
				if err != nil {
					fmt.Printf("Could not get global orbit properties")
					return
				}

				fmt.Printf("Connecting to MySQL: %s\n", dburi)

				dbLog := containrunner.DbLog{}
				err = dbLog.Init(dburi)
				if err != nil {
					fmt.Printf("Could not connect to mysql. err: %+v", err)
					return
				}

				fmt.Printf("Connecting to AMQP: %s\n", globalConfiguration.AMQPUrl)
				events := new(containrunner.RabbitMQQueuer)
				connected := events.Init(globalConfiguration.AMQPUrl, "orbitctl.deployment_events_persistent_storage")

				if !connected {
					fmt.Printf("Could not connect to message broker")
				}

				for event := range events.GetReceiveredEventChannel() {
					err = dbLog.StoreEvent(event)
					if err != nil {
						fmt.Printf("Error storing event %s: %+v\n", event, err)
					}
				}

			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "dburi",
					Usage:  "Required: Connection uri to database: username:password@protocol(address)/dbname",
					EnvVar: "ORBITCTL_DBURI",
				},
			},
		})
}
