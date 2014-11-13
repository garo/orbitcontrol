package main

import (
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"strings"
)

func init() {
	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "daemon",
			Usage: "Start watching for required containers, checks for service availability and updates haproxy",
			Action: func(c *cli.Context) {

				containrunnerInstance.Start()
				containrunnerInstance.Wait()

			},
			Before: func(c *cli.Context) error {
				if c.String("machine-address") == "" {
					cli.ShowSubcommandHelp(c)
					return errors.New("machine-address missing")
				}

				if c.String("machine-tags") == "" {
					cli.ShowSubcommandHelp(c)
					return errors.New("machine-tags missing")
				}

				containrunnerInstance.MachineAddress = c.String("machine-address")
				containrunnerInstance.Tags = strings.Split(c.String("machine-tags"), ",")
				containrunnerInstance.CheckIntervalInMs = c.Int("check-interval-in-ms")
				containrunnerInstance.HAProxySettings.HAProxyConfigPath = c.String("haproxy-config-path")
				containrunnerInstance.HAProxySettings.HAProxyConfigName = c.String("haproxy-config-name")
				containrunnerInstance.HAProxySettings.HAProxyBinary = c.String("haproxy-binary")
				containrunnerInstance.HAProxySettings.HAProxyReloadCommand = c.String("haproxy-reload-command")
				containrunnerInstance.HAProxySettings.HAProxySocket = c.String("haproxy-socket")

				fmt.Printf("Settings: %+v\n", containrunnerInstance)
				return nil
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "machine-address",
					Usage:  "Required: Machine external ip into which other machines can connect to, usually ip of eth0. Does not mean public ip in AWS.",
					EnvVar: "ORBITCTL_MACHINE_ADDRESS",
				},
				cli.StringFlag{
					Name:  "machine-tags",
					Usage: "Required: Comma separated list of tags this machine belongs to",
				},
				cli.IntFlag{
					Name:  "check-interval-in-ms",
					Value: 2000,
					Usage: "Delay of checks to each monitored service",
				},
				cli.StringFlag{
					Name:   "haproxy-config-path",
					Value:  "/etc/haproxy",
					Usage:  "Path of the haproxy config file. This path will also host backups of the config",
					EnvVar: "ORBITCTL_HAPROXY_CONFIG_PATH",
				},
				cli.StringFlag{
					Name:  "haproxy-config-name",
					Value: "haproxy.cfg",
					Usage: "Name of the generated haproxy config file",
				},
				cli.StringFlag{
					Name:  "haproxy-binary",
					Value: "/usr/sbin/haproxy",
					Usage: "Full path to haproxy binary",
				},
				cli.StringFlag{
					Name:  "haproxy-reload-command",
					Value: "/etc/init.d/haproxy reload",
					Usage: "Command to reload haproxy",
				},
				cli.StringFlag{
					Name:  "haproxy-socket",
					Value: "/var/run/haproxy/admin*.sock",
					Usage: "HAProxy admin socket. Can use wildcards to contact multiple sockets when haproxy uses nbproc > 1",
				},
			},
		})
}
