package main

import (
	"strings"
)

var (
	cmdDaemon = &Command{
		Name:        "daemon",
		Summary:     "Start watching for required containers, checks for service availability and updates haproxy",
		Usage:       "",
		Description: "",
		Run:         runDaemon,
	}

	machineTags string
)

func init() {
	cmdDaemon.Flags.StringVar(&containrunnerInstance.MachineAddress, "machine-address", "", "Machine external ip into which other machines can connect to, usually ip of eth0. Does not mean public ip in AWS.")
	cmdDaemon.Flags.IntVar(&containrunnerInstance.CheckIntervalInMs, "check-interval-in-ms", 300, "Delay of checks to each monitored service")
	cmdDaemon.Flags.StringVar(&containrunnerInstance.HAProxySettings.HAProxyConfigPath, "haproxy-config-path", "/tmp", "Path of the haproxy config file. This path will also host backups of the config")
	cmdDaemon.Flags.StringVar(&containrunnerInstance.HAProxySettings.HAProxyConfigName, "haproxy-config-name", "haproxy-orbit.cfg", "Name of the generated haproxy config file")
	cmdDaemon.Flags.StringVar(&containrunnerInstance.HAProxySettings.HAProxyBinary, "haproxy-binary", "/usr/sbin/haproxy", "Full path to haproxy binary")
	cmdDaemon.Flags.StringVar(&machineTags, "machine-tags", "", "Comma separated list of tags this machine belongs to")
}

func runDaemon(args []string) (exit int) {

	if containrunnerInstance.MachineAddress == "" {
		printCommandUsageByName("daemon", "machine-address")
		return 1
	}

	if machineTags == "" {
		printCommandUsageByName("daemon", "machine-tags")
		return 1
	}

	containrunnerInstance.Tags = strings.Split(machineTags, ",")
	containrunnerInstance.EtcdEndpoints = GetEtcdEndpoints()

	//fmt.Printf("Starting containrunner with config %+v\n", containrunnerInstance)
	containrunnerInstance.Start()
	containrunnerInstance.Wait()
	return 0
}
