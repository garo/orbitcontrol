package main

import (
	"fmt"
	"os"
)

var (
	cmdTags = &Command{
		Name:        "tags",
		Summary:     "List known tags",
		Usage:       "",
		Description: "",
		Run:         runTags,
	}
)

func init() {
}

func runTags(args []string) (exit int) {

	tags, err := containrunnerInstance.GetKnownTags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting tags: %+v\n", err)
		return 1
	}
	fmt.Printf("%+v\n", tags)

	return 0
}
