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
			Name:  "webserver",
			Usage: "Starts local webserver",
			Action: func(c *cli.Context) {
				fmt.Println("Starting webserver. This really doesn't do anything yet")
				var webserver containrunner.Webserver
				webserver.Containrunner = &containrunnerInstance
				webserver.Start()

				time.Sleep(9223372036854775807) // Not my proudest moment
			},
		})
}
