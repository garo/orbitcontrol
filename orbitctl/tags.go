package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
)

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "tags",
			Usage: "List known machine tags",
			Action: func(c *cli.Context) {
				tags, err := containrunnerInstance.GetKnownTags()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting tags: %+v\n", err)
					os.Exit(1)
				}
				fmt.Printf("%+v\n", tags)
			},
		})
}
