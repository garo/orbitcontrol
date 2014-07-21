package main

import (
	"fmt"
	"os"
)

var (
	cmdTag = &Command{
		Name:        "tag",
		Summary:     "Tag service to a tag",
		Usage:       "orbitctl tag <service> to <tag>",
		Description: "",
		Run:         runTag,
	}
)

func init() {
}

func runTag(args []string) (exit int) {

	if len(args) != 3 || args[1] != "to" {
		fmt.Fprintf(os.Stderr, "Usage: orbitctl tag <service> to <tag>\n")
		return 1
	}

	service := args[0]
	tag := args[2]

	err := containrunnerInstance.TagServiceToTag(service, tag, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error on tagging service %s to tag %s: %+v\n", service, tag, err)
		return 1
	} else {
		fmt.Fprintf(os.Stderr, "Tagged service %s to tag %s\n", service, tag)
	}

	return 0
}
