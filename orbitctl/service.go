package main

import (
	"bufio"
	"code.google.com/p/goauth2/oauth"
	"errors"
	"fmt"
	"github.com/garo/orbitcontrol/containrunner"
	"github.com/google/go-github/github"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	cmdService = &Command{
		Name:    "service",
		Summary: "Control services",
		Usage: `
	service <name> set revision <revision>
	service <name>
`,
		Description: "",
		Run:         runService,
	}
)

func init() {
}

func runService(args []string) (exit int) {

	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: "a91a73cc3bf597ac6b56f45ef11ed98937ab467a"},
	}

	client := github.NewClient(t.Client())

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Need service name as first argument\n")
		return 1
	}

	name := args[0]
	serviceConfiguration, err := containrunnerInstance.GetServiceByName(name, nil)
	if err != nil {
		panic(err)
	}

	if serviceConfiguration.SourceControl != nil && serviceConfiguration.SourceControl.CIUrl != "" {
		fmt.Printf("Continuous Integration server url for this service: %s\n", serviceConfiguration.SourceControl.CIUrl)
	}

	reader := bufio.NewReader(os.Stdin)

	switch {
	case len(args) == 1:
		fmt.Printf("\x1b[1mService %s is currently running revision %s\x1b[0m\n", name, serviceConfiguration.GetRevision())
		commit, err := GetCommitInfo(serviceConfiguration.SourceControl, serviceConfiguration.GetRevision(), client)
		if err != nil {
			fmt.Printf("Error! Unable to get source control information on revision.\nError: %+v\n", err)
			return 1
		} else {
			PrintCommitInfo(commit)
		}

		fmt.Printf("\n")
		if serviceConfiguration.Revision != nil && !serviceConfiguration.Revision.DeploymentTime.IsZero() {
			fmt.Printf("\x1b[1mDeployment was done at %s (%s ago)\x1b[0m\n", serviceConfiguration.Revision.DeploymentTime, time.Since(serviceConfiguration.Revision.DeploymentTime))
		}

		if serviceConfiguration.SourceControl != nil {
			commits, err := GetNewerCommits(serviceConfiguration.SourceControl, commit, client)
			if err != nil {
				fmt.Printf("Error! Unable to get source control information on newer revisions.\nError: %+v\n", err)
				return 1
			}

			var deployable_commits = make([]github.RepositoryCommit, 0)
			if len(commits) > 0 {
				for _, commit := range commits {
					exists, _, err := containrunner.VerifyContainerExistsInRepository(serviceConfiguration.Container.Config.Image, *commit.SHA)
					if err != nil {
						fmt.Printf("Error! Unable to get container registry information on newer revisions.\nError: %+v\n", err)
						return 1
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
			}
		}

	case len(args) == 4 && args[1] == "set" && args[2] == "revision" && args[3] != "":
		revision := args[3]

		if serviceConfiguration.SourceControl != nil {
			commit, err := GetCommitInfo(serviceConfiguration.SourceControl, revision, client)
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
				return 1
			} else if serviceConfiguration.Revision != nil {
				fmt.Printf("Previous revision: %s\n", serviceConfiguration.Revision.Revision)
			} else {
				fmt.Printf("Service %s previously had no dynamic revision!\nStatic revision is: %s\n", name, containrunner.GetContainerImageNameWithRevision(serviceConfiguration, ""))
			}

		} else {
			fmt.Printf("Warning! Unable to get source control information on revision. SourceControl data not set for service\n")
		}

		fmt.Printf("Setting service %s revision to %s\n", name, revision)
		fmt.Printf("\n")

		image_name := containrunner.GetContainerImageNameWithRevision(serviceConfiguration, revision)
		exists, last_update, err := containrunner.VerifyContainerExistsInRepository(image_name, "")
		if err != nil {
			panic(err)
		}

		if exists == false {
			fmt.Printf("Error! Unable to find correct container from repository for this revision!\nMissing container name: %s\n", image_name)
			return 1
		}

		diff := time.Since(time.Unix(last_update, 0))
		fmt.Printf("The container %s you are about to deploy was last updated at %s ago\n", revision, diff)

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
		err = containrunnerInstance.SetServiceRevision(name, serviceRevision, nil)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Monitoring deployment progress (New deployment has been committed, you can press Ctrl-C to stop monitoring)\n")
		fmt.Printf("Full deployment takes around two minutes\n")

		for true {
			time.Sleep(time.Second * 1)
			old, updated, _ := CheckDeploymentProgress(name, revision)
			fmt.Printf("Servers with old revision: %d, servers with new revision: %d... \r", len(old), len(updated))
			if len(old) == 0 && len(updated) > 0 {
				break
			}
		}

		fmt.Printf("\nAll servers updated\n")

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
	endpoints, err := containrunnerInstance.GetHAProxyEndpointsForService(service_name)
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
