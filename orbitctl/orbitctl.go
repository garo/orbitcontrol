// This file has copied some structures from the coreos/fleet project
// available at https://github.com/coreos/fleet/blob/master/fleetctl/fleetctl.go
// The fleet project uses APACHE 2.0 License

package main

import (
	"flag"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/coreos/go-etcd/etcd"
	"github.com/garo/orbitcontrol/containrunner"
	"log"
	"os"
	"strings"
	"text/tabwriter"
)

var builddate string

const (
	cliName = "orbitctl"

	cliDescription = "orbitcrl is a tool to command and distribute services inside containers into sets of machines"
)

var (
	out *tabwriter.Writer
	app *cli.App = cli.NewApp()

	globalFlags = struct {
		Debug        bool
		EtcdEndpoint string
		EtcdBasePath string
		Force        bool
	}{}

	containrunnerInstance containrunner.Containrunner
)

type Command struct {
	Name        string       // Name of the Command and the string to use to invoke it
	Summary     string       // One-sentence summary of what the Command does
	Usage       string       // Usage options/arguments
	Description string       // Detailed description of command
	Flags       flag.FlagSet // Set of flags associated with this command

	Run func(args []string) int // Run a command with the given arguments, return exit status

}

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
	app.EnableBashCompletion = true

}

func main() {
	app.Name = "orbitctl"
	app.Usage = cliDescription
	app.Version = builddate

	etcdEndpointFlag := cli.StringFlag{
		Name:   "etcd-endpoint",
		Value:  "http://etcd:4001",
		Usage:  "Etcd server endpoint as http://host:port[,http://host:port] string",
		EnvVar: "ORBITCTL_ETCD_ENDPOINT",
	}
	etcdBasePathFlag := cli.StringFlag{
		Name:   "etcd-base-path",
		Value:  "/orbit",
		Usage:  "Keyspace for orbit control data in etcd",
		EnvVar: "ORBITCTL_ETCD_BASE_PATH",
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "Print out more debug information to stderr",
			EnvVar: "ORBITCTL_DEBUG",
		},
		cli.BoolFlag{
			Name:   "force",
			Usage:  "Force, don't ask questions",
			EnvVar: "ORBITCTL_FORCE",
		},
		cli.StringFlag{
			Name:   "github-token",
			Usage:  "Github OAuth2 token for accessing github commit info",
			EnvVar: "ORBITCTL_GITHUB_TOKEN",
		},
		cli.BoolFlag{
			Name:   "disable-amqp",
			Usage:  "Disable amqp",
			EnvVar: "ORBITCTL_DISABLE_AMQP",
		},
		etcdBasePathFlag,
		etcdEndpointFlag,
	}

	app.Before = func(c *cli.Context) error {

		containrunnerInstance.EtcdEndpoints = strings.Split(c.String("etcd-endpoint"), ",")
		containrunnerInstance.EtcdBasePath = c.String("etcd-base-path")

		globalFlags.Force = c.Bool("force")

		if c.IsSet("debug") {
			fmt.Fprintf(os.Stderr, "Turning etcd logging on")
			etcd.SetLogger(log.New(os.Stderr, "go-etcd", log.LstdFlags))
		}

		if c.IsSet("disable-amqp") {
			fmt.Fprintf(os.Stderr, "Disabling AMQP due to --disable-amqp option\n")
			containrunnerInstance.DisableAMQP = true
		}

		// dblog is a special case which doesn't want the containrunner to be initiated.
		if len(c.Args()) > 0 && c.Args()[0] != "dblog" {
			containrunnerInstance.Init()
		}

		return nil
	}

	app.Run(os.Args)

}

// getFlagsFromEnv parses all registered flags in the given flagset,
// and if they are not already set it attempts to set their values from
// environment variables. Environment variables take the name of the flag but
// are UPPERCASE, have the given prefix, and any dashes are replaced by
// underscores - for example: some-flag => PREFIX_SOME_FLAG
func getFlagsFromEnv(prefix string, fs *flag.FlagSet) {
	alreadySet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		alreadySet[f.Name] = true
	})
	fs.VisitAll(func(f *flag.Flag) {
		if !alreadySet[f.Name] {
			key := strings.ToUpper(prefix + "_" + strings.Replace(f.Name, "-", "_", -1))
			val := os.Getenv(key)
			if val != "" {
				fs.Set(f.Name, val)
			}
		}

	})
}
