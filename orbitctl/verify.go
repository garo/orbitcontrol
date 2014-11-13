package main

import (
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"os"
)

var verifyHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}
USAGE:
   {{.Name}} [path to local directory tree]

`

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "verify",
			Usage: "Verifies etcd configuration against local copy",
			Flags: nil,
			Before: func(c *cli.Context) error {
				if c.Args().First() == "" {
					cli.HelpPrinter(verifyHelpTemplate, c.App)
					return errors.New("local path is missing")
				}

				return nil
			},
			Action: func(c *cli.Context) {
				path := c.Args()[1]
				fmt.Printf("verifying against %s\n", path)
				err := containrunnerInstance.VerifyAgainstLocalDirectory(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "configuration did not match.\nError: %+v\n", err)
					os.Exit(1)
				} else {
					fmt.Fprintf(os.Stderr, "All ok\n")
				}
			},
		})
}
