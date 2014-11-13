package main

import (
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"os"
)

var importHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}
USAGE:
   {{.Name}} [import path to orbit configuration]

`

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "import",
			Usage: "Import orbit configurations from directory tree",
			Flags: nil,
			Before: func(c *cli.Context) error {
				if c.Args().First() == "" {
					cli.HelpPrinter(importHelpTemplate, c.App)
					return errors.New("import path is missing")
				}

				return nil
			},
			Action: func(c *cli.Context) {
				path := c.Args()[1]
				fmt.Printf("import from %s\n", path)

				orbitConfiguration, err := containrunnerInstance.LoadOrbitConfigurationFromFiles(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
					os.Exit(1)
				}

				err = containrunnerInstance.UploadOrbitConfigurationToEtcd(orbitConfiguration, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
					os.Exit(1)
				}
			},
		})
}
