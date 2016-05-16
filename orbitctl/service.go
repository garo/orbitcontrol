package main

/*
import (
	"bufio"
	"code.google.com/p/goauth2/oauth"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/garo/orbitcontrol/containrunner"
	"github.com/google/go-github/github"
	"regexp"
	"strings"
	"time"
		"os"

)*/

import (
	"bufio"
	"code.google.com/p/goauth2/oauth"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/garo/orbitcontrol/containrunner"
	"github.com/google/go-github/github"
	"os"
	"os/user"
	"regexp"
	"sort"
	"strings"
	"time"
)

var serviceHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}.

USAGE:
   {{.Name}} [service name]
			Displays service info

   {{.Name}} [service name] set revision <revision>
			Set service revision

   {{.Name}} [service name] set revision <revision> on machine <ip>
   			Set service revision for particular machine


`

func init() {

	serviceApp := cli.NewApp()
	serviceApp.Name = ""
	serviceApp.EnableBashCompletion = true
	serviceApp.HideHelp = true
	serviceApp.Action = func(c *cli.Context) {
		if len(c.Args()) == 0 {
			t := &oauth.Transport{
				Token: &oauth.Token{AccessToken: c.GlobalString("github-token")},
			}

			githubClient := github.NewClient(t.Client())

			getServiceInfo(c.App.Name, githubClient)
		} else {
			serviceApp.RunAsSubcommand(c)
		}

	}
	serviceApp.Commands = []cli.Command{
		{
			Name: "relaunch",
			Action: func(c *cli.Context) {
				fmt.Printf("Sending relaunch signal for container %s\n", c.App.Name)
				deploymentEvent := containrunner.DeploymentEvent{}
				deploymentEvent.Action = "RelaunchContainer"
				deploymentEvent.Service = c.App.Name
				deploymentEvent.Jitter = 30
				event := containrunner.NewOrbitEvent(deploymentEvent)
				if containrunnerInstance.Events != nil {
					containrunnerInstance.Events.PublishOrbitEvent(event)
				} else {
					fmt.Printf("Error, Events subsystem not enabled. Maybe RabbitMQ is not configured?")
				}
			},
		},
		{
			Name:     "set",
			HideHelp: true,

			Action: func(c *cli.Context) {

				if c.Bool(cli.BashCompletionFlag.Name) {
					c.App.BashComplete(c)
					return
				}

				if len(c.Args()) == 0 {
					cli.HelpPrinter(serviceHelpTemplate, c.App)
				}

			},
			Subcommands: []cli.Command{
				{
					Name:     "revision",
					HideHelp: true,
					Before: func(c *cli.Context) error {
						if c.Bool(cli.BashCompletionFlag.Name) || c.Args()[len(c.Args())-1] == "--generate-bash-completion" {
							if len(c.Args()) == 2 {
								fmt.Println("to")
							} else if len(c.Args()) == 3 {
								fmt.Println("machine")
							}
							return errors.New("")
						}

						return nil
					},
					Action: func(c *cli.Context) {
						name := c.App.Name[0:strings.Index(c.App.Name, " ")]

						if len(c.Args()) == 0 || c.Args()[0] != "revision" {
							cli.HelpPrinter(serviceHelpTemplate, c.App)
							return
						}

						revision := c.Args()[1]

						machineAddress := ""
						if len(c.Args()) == 5 && c.Args()[3] == "machine" {
							machineAddress = c.Args()[4]
						}

						t := &oauth.Transport{
							Token: &oauth.Token{AccessToken: c.GlobalString("github-token")},
						}

						githubClient := github.NewClient(t.Client())
						if githubClient == nil {
							fmt.Printf("Error getting github client\n")
						}

						retval, serviceConfiguration := getServiceInfo(name, githubClient)
						if retval != 0 {
							os.Exit(retval)
						}

						deploymentEvent := containrunner.DeploymentEvent{}
						deploymentEvent.Action = "SetRevision"
						deploymentEvent.Service = name
						deploymentEvent.Revision = revision
						deploymentEvent.MachineAddress = machineAddress
						user, err := user.Current()
						if err == nil {
							deploymentEvent.User = user.Username
						}
						event := containrunner.NewOrbitEvent(deploymentEvent)
						if containrunnerInstance.Events != nil {
							containrunnerInstance.Events.PublishOrbitEvent(event)
						}

						retval = setServiceRevision(name, revision, machineAddress, serviceConfiguration, githubClient)
						if retval != 0 {
							os.Exit(retval)
						}
						deploymentEvent.Action = "DeployCompleted"
						event = containrunner.NewOrbitEvent(deploymentEvent)
						if containrunnerInstance.Events != nil {
							containrunnerInstance.Events.PublishOrbitEvent(event)
						} else {
							fmt.Printf("Error, Events subsystem not enabled. Maybe RabbitMQ is not configured?")
						}

					},
				},
			},
		},
	}

	app.Commands = append(app.Commands,
		cli.Command{
			Name:  "service",
			Usage: "Control services",
			Action: func(c *cli.Context) {

				if len(c.Args()) == 0 {
					cli.HelpPrinter(serviceHelpTemplate, c.App)
					return
				}

				serviceApp.Flags = c.App.Flags
				serviceApp.HideHelp = true
				serviceApp.Name = c.Args()[0]
				serviceApp.Run(c.Args())
			},
			BashComplete: func(c *cli.Context) {
				services, err := containrunnerInstance.GetAllServices(nil)
				if err != nil {
					fmt.Printf("Error: %+v\n", err)
					return
				}
				if len(c.Args()) > 0 {
					newArgs := append(c.Args(), "--generate-bash-completion")
					serviceApp.Run(newArgs)
					return
				}

				for service, _ := range services {
					fmt.Println(service)
				}
			},
		})
}

func getServiceInfo(name string, githubClient *github.Client) (exit int, serviceConfiguration containrunner.ServiceConfiguration) {

	serviceConfiguration, err := containrunnerInstance.GetServiceByName(name, nil, "")
	if err != nil {
		log.Fatalf("Service %s not found", name)
	}

	var commit *github.RepositoryCommit

	revision := serviceConfiguration.GetRevision()
	if revision != "" {
		fmt.Printf("\x1b[1mService %s is currently running revision %s\x1b[0m\n", name, revision)
	}

	if serviceConfiguration.SourceControl != nil && serviceConfiguration.SourceControl.Origin != "" {

		commit, err = GetCommitInfo(serviceConfiguration.SourceControl, serviceConfiguration.GetRevision(), githubClient)
		if err != nil {
			fmt.Printf("Error! Unable to get source control information on revision.\nError: %+v\n", err)
			if globalFlags.Force == false {
				return 1, serviceConfiguration
			}
		} else {
			PrintCommitInfo(commit)
		}
	} else {
		fmt.Printf("No SourceControl Origin set for this service, thus there's no commit info available.")
	}

	if serviceConfiguration.SourceControl != nil && serviceConfiguration.SourceControl.CIUrl != "" {
		fmt.Printf("Continuous Integration server url for this service: %s\n", serviceConfiguration.SourceControl.CIUrl)
	}

	fmt.Printf("\n")
	if serviceConfiguration.Revision != nil && !serviceConfiguration.Revision.DeploymentTime.IsZero() {
		fmt.Printf("\x1b[1mDeployment was done at %s (%s ago)\x1b[0m\n", serviceConfiguration.Revision.DeploymentTime, time.Since(serviceConfiguration.Revision.DeploymentTime))
	}

	if serviceConfiguration.SourceControl != nil && serviceConfiguration.SourceControl.Origin != "" && commit != nil {
		commits, err := GetNewerCommits(serviceConfiguration.SourceControl, commit, githubClient)
		if err != nil {
			fmt.Printf("Error! Unable to get source control information on newer revisions.\nError: %+v\n", err)
			return 1, serviceConfiguration
		}

		var deployable_commits = make([]github.RepositoryCommit, 0)
		if len(commits) > 0 {
			for _, commit := range commits {
				fmt.Printf("Commit: %s\n", *commit.SHA)
				exists, _, err := containrunner.VerifyContainerExistsInRepository(serviceConfiguration.Container.Config.Image, *commit.SHA)
				if err != nil {
					fmt.Printf("Error! Unable to get container registry information on newer revision for commit %s\nError: %+v\n", *commit.SHA, err)
					return 1, serviceConfiguration
				}
				if exists && *commit.SHA != serviceConfiguration.GetRevision() {
					deployable_commits = append(deployable_commits, commit)
				}
			}

			// Reverse sort
			for i, j := 0, len(deployable_commits)-1; i < j; i, j = i+1, j-1 {
				deployable_commits[i], deployable_commits[j] = deployable_commits[j], deployable_commits[i]
			}

			fmt.Printf("\x1b[1mThere are %d newer commits than the currently deployed revision, from which %d could be deployed (from old to new):\x1b[0m\n", len(commits), len(deployable_commits))
			for _, commit := range deployable_commits {
				fmt.Printf("\n")
				PrintCommitInfo(&commit)
			}

			if len(deployable_commits) > 0 {
				fmt.Printf("\nYou can deploy the newest commit with this command: \x1b[1morbitctl service %s set revision %s\x1b[0m\n\n", name, *deployable_commits[len(deployable_commits)-1].SHA)
			}
		} else {
			fmt.Printf("Service %s is running the latest revision (%s) according to version control.\n", name, commit)
		}
	}

	return 0, serviceConfiguration
}

func setServiceRevision(name string, revision string, machineAddress string, serviceConfiguration containrunner.ServiceConfiguration, githubClient *github.Client) int {
	reader := bufio.NewReader(os.Stdin)

	if serviceConfiguration.SourceControl != nil && serviceConfiguration.SourceControl.Origin != "" {
		commit, err := GetCommitInfo(serviceConfiguration.SourceControl, revision, githubClient)
		if err != nil {
			fmt.Printf("Error! Unable to get source control information on revision.\nError: %+v\n", err)
			fmt.Printf("Can't deploy something which can't exists in the source control system!\n")

			return 1
		} else {
			PrintCommitInfo(commit)

			// GitHub allows us to query for a revision with just an unique prefix, so we can now obtain the full revision tag
			if revision != *commit.SHA {
				revision = *commit.SHA
			}
		}

		if serviceConfiguration.GetRevision() == revision {
			fmt.Printf("Service %s is already at revision %s\n", name, revision)
			return 0
		} else if serviceConfiguration.Revision != nil {
			fmt.Printf("Previous revision: %s\n", serviceConfiguration.Revision.Revision)
		} else {
			fmt.Printf("Service %s previously had no dynamic revision!\nStatic revision is: %s\n", name, containrunner.GetContainerImageNameWithRevision(serviceConfiguration, ""))
		}

	} else {
		fmt.Printf("Warning! Unable to get source control information on revision. SourceControl data not set for service\n")
	}

	if serviceConfiguration.Container == nil {
		fmt.Printf("Error! The service %s configuration in incomplete! Missign Container data\n", name)
		return 1
	}

	image_name := containrunner.GetContainerImageNameWithRevision(serviceConfiguration, revision)
	exists, last_update, err := containrunner.VerifyContainerExistsInRepository(image_name, "")
	if err != nil {
		fmt.Printf("Container %s not found from local repository!\n", image_name)
		return 1
	}

	if exists == false {
		fmt.Printf("Error! Unable to find correct container from repository for this revision!\nMissing container name: %s\n", image_name)
		return 1
	}

	diff := time.Since(time.Unix(last_update, 0))
	if last_update == 0 {
		fmt.Printf("Last update time for container is not known due to registry v2\n")
	} else {
		fmt.Printf("The container %s you are about to deploy was last updated at %s ago\n", revision, diff)
	}

	if machineAddress != "" {
		fmt.Printf("Setting service %s revision to %s for machine ip %s\n\n", name, revision, machineAddress)
	} else {
		fmt.Printf("Setting service %s revision to %s\n\n", name, revision)
	}

	if globalFlags.Force == false {
		fmt.Printf("Are you sure you want to deploy %s with this revision into production? (y/N) ", name)
		bytes, _ := reader.ReadBytes('\n')
		if bytes[0] != 'y' && bytes[0] != 'Y' {
			fmt.Printf("Abort!\n")
			return 1
		}
	}

	var serviceRevision = containrunner.ServiceRevision{
		Revision:       revision,
		DeploymentTime: time.Now(),
	}

	if machineAddress != "" {
		err = containrunnerInstance.SetServiceRevisionForMachine(name, serviceRevision, machineAddress, nil)
	} else {
		err = containrunnerInstance.SetServiceRevision(name, serviceRevision, nil)
	}
	if err != nil {
		panic(err)
	}

	if serviceConfiguration.Checks == nil || len(serviceConfiguration.Checks) == 0 {
		fmt.Printf("Service has no checks, so deployment progress can't be monitored\n")
	} else {
		fmt.Printf("Monitoring deployment progress (New deployment has been committed, you can press Ctrl-C to stop monitoring)\n")
		fmt.Printf("Full deployment takes around two minutes\n")

		if machineAddress == "" {
			count := 0
			for true {
				count = count + 1
				time.Sleep(time.Second * 1)
				old, updated, _ := CheckDeploymentProgress(name, revision)
				fmt.Printf("Servers with old revision: %d, servers with new revision: %d... \r", len(old), len(updated))
				if count >= 20 && count%10 == 0 {
					fmt.Printf("\nServers with still old revision: ")
					sort.Strings(old)
					for _, ip := range old {
						fmt.Printf("%s ", ip)
					}
					fmt.Printf("\n")
				}
				if len(old) == 0 && len(updated) > 0 {
					break
				}
			}

			fmt.Printf("\nAll servers updated\n")
		} else {
			fmt.Printf("Deploying to machine %s. Monitoring not implemented for single machien deployment updates.\n", machineAddress)
		}
	}

	return 0
}

func GetCommitInfo(sc *containrunner.SourceControl, revision string, client *github.Client) (*github.RepositoryCommit, error) {

	// github.com/Applifier/comet
	var orgRegexp = regexp.MustCompile("github.com/(.+?)/(.+)")

	m := orgRegexp.FindStringSubmatch(sc.Origin)
	if m == nil {
		return nil, errors.New("Invalid SourceControl origin: " + sc.Origin)
	}

	commit, _, err := client.Repositories.GetCommit(m[1], m[2], revision)
	if err != nil {
		return nil, err
	}

	return commit, nil
}

func GetNewerCommits(sc *containrunner.SourceControl, newer_than *github.RepositoryCommit, client *github.Client) ([]github.RepositoryCommit, error) {

	// github.com/Applifier/comet
	var orgRegexp = regexp.MustCompile("github.com/(.+?)/(.+)")

	m := orgRegexp.FindStringSubmatch(sc.Origin)
	if m == nil {
		return nil, errors.New("Invalid SourceControl origin: " + sc.Origin)
	}

	options := github.CommitsListOptions{
		Since: *newer_than.Commit.Author.Date,
	}

	commits, _, err := client.Repositories.ListCommits(m[1], m[2], &options)
	if err != nil {
		return nil, err
	}

	if len(commits) > 1 {
		commits = commits[0 : len(commits)-1]
	}

	return commits, nil
}

func PrintCommitInfo(commit *github.RepositoryCommit) {
	fmt.Printf("Commit %s\n", *commit.SHA)
	fmt.Printf("Author: %s <%s>\n", *commit.Commit.Author.Name, *commit.Commit.Author.Email)
	fmt.Printf("Date: %s (%s ago)\n", *commit.Commit.Author.Date, time.Since(*commit.Commit.Author.Date))
	fmt.Printf("\n")

	for _, line := range strings.Split(*commit.Commit.Message, "\n") {
		fmt.Printf("\t%s\n", line)
	}
}

func CheckDeploymentProgress(service_name string, desired_revision string) (old []string, updated []string, err error) {
	endpoints, err := containrunnerInstance.GetEndpointsForService(service_name)
	if err != nil {
		panic(err)
	}

	for ip, endpointInfo := range endpoints {
		if endpointInfo != nil && endpointInfo.Revision == desired_revision {
			updated = append(updated, ip)
		} else {
			old = append(old, ip)
		}
	}

	return old, updated, nil
}
