package main

import (
	"fmt"
	"os"
)

var (
	cmdImport = &Command{
		Name:        "import",
		Summary:     "Import orbit configurations from directory tree",
		Usage:       "",
		Description: "",
		Run:         runImport,
	}
)

func init() {
}

func runImport(args []string) (exit int) {

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Need path to orbit configuration")
		return 1
	}

	orbitConfiguration, err := containrunnerInstance.LoadOrbitConfigurationFromFiles(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v", err)
		return 1
	}

	err = containrunnerInstance.UploadOrbitConfigurationToEtcd(orbitConfiguration, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v", err)
		return 1
	}

	return 0
}
