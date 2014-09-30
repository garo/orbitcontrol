package main

import (
	"fmt"
	"os"
)

var (
	cmdVerify = &Command{
		Name:        "verify",
		Summary:     "Verifies etcd configuration against local copy",
		Usage:       "<local dir>",
		Description: "",
		Run:         runVerify,
	}
)

func init() {
}

func runVerify(args []string) (exit int) {

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: orbitctl verify <local dir>\n")
		return 1
	}

	dir := args[0]

	err := containrunnerInstance.VerifyAgainstLocalDirectory(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration did not match. Error: %+v", err)
		return 1
	} else {
		fmt.Fprintf(os.Stderr, "All ok\n")
	}

	return 0
}
