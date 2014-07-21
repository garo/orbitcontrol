package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	cmdImport = &Command{
		Name:        "import",
		Summary:     "Import services from file",
		Usage:       "",
		Description: "",
		Run:         runImport,
	}
)

func init() {
}

func runImport(args []string) (exit int) {
	containrunnerInstance.EtcdEndpoints = GetEtcdEndpoints()

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Need at least one .json file as argument")
		return 1
	}

	for _, arg := range args {
		pos := strings.Index(arg, ".json")
		if pos == -1 {
			fmt.Fprintf(os.Stderr, "File (%s) must end in .json\n", arg)
			return 1
		}
		name := filepath.Base(arg[0:pos])
		fullpath, err := filepath.Abs(arg)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Could not find file %s\n", fullpath)
		}
		fmt.Fprintf(os.Stdout, "Importing service %s from file %s\n", name, fullpath)

		err = containrunnerInstance.ImportServiceFromFile(name, fullpath, nil)
		if err != nil {
			return 1
		}

	}

	return 0
}
