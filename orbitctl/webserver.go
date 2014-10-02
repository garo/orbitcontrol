package main

import (
	"github.com/garo/orbitcontrol/containrunner"
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
	webserver.Start(false)

	return 0
}
