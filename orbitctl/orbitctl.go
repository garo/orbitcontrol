// This file has copied some structures from the coreos/fleet project
// available at https://github.com/coreos/fleet/blob/master/fleetctl/fleetctl.go
// The fleet project uses APACHE 2.0 License

package main

import (
	"flag"
	"text/tabwriter"
)

var (
	out      *tabwriter.Writer
	commands []*Command

	globalFlagset = flag.NewFlagSet("orbitctl", flag.ExitOnError)

	globalFlags = struct {
		Debug         bool
		EtcdEndpoint  string
		EtcdKeyPrefix string
	}{}
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
	commands = []*Command{}

	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information to stderr")
	globalFlagset.StringVar(&globalFlags.EtcdEndpoint, "etcd-endpoint", "etcd:4001", "Etcd server endpoint as http://host:port[,http://host:port] string")
	globalFlagset.StringVar(&globalFlags.EtcdKeyPrefix, "etcd-key-prefix", "", "Keyspace for fleet data in etcd")

}

func main() {
	// parse global arguments
	globalFlagset.Parse(os.Args[1:])

	var args = globalFlagset.Args()

	getFlagsFromEnv(cliName, globalFlagset)

	// no command specified - trigger help
	if len(args) < 1 {
		args = append(args, "help")
	}

	var cmd *Command

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(args[1:]); err != nil {
				fmt.Println(err.Error())
				os.Exit(2)
			}
			break
		}
	}
}
