package main

import (
	"github.com/garo/orbitcontrol/containrunner"
	"time"
)

var (
	cmdWebserver = &Command{
		Name:        "webserver",
		Summary:     "Start local webserver",
		Usage:       "",
		Description: "",
		Run:         runWebserver,
	}
)

func runWebserver(args []string) (exit int) {

	var webserver containrunner.Webserver
	webserver.Containrunner = &containrunnerInstance
	webserver.Start()

	time.Sleep(9223372036854775807) // Not my proudest moment
	return 0
}
