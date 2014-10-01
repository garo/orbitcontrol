package containrunner

import (
	"encoding/json"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type ContainerConfiguration struct {
	HostConfig docker.HostConfig
	Config     docker.Config
}

type ContainerDetails struct {
	docker.APIContainers
	Container *docker.Container
}

type ContainerLogEvent struct {
	Event          string
	ContainerImage string
	ContainerName  string
}

func GetServiceConfigurationString() string {
	return ""
}

func GetDockerClient() *docker.Client {
	endpoint := "unix:///var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}
	return client
}

func FindMatchingContainers(existing_containers []ContainerDetails, required_service ServiceConfiguration) (found_containers []ContainerDetails, remaining_containers []ContainerDetails) {
	var imageRegexp = regexp.MustCompile("(.+):")

	for _, container_details := range existing_containers {
		found := true

		// Support the Revision code path where the revision overrides the image (which contains both container and revision tag)
		// set in the static Container.Config.Image
		if required_service.Revision != nil {
			m := imageRegexp.FindStringSubmatch(required_service.Container.Config.Image)
			image := m[1] + ":" + required_service.Revision.Revision

			if container_details.Container.Config.Image != image {
				remaining_containers = append(remaining_containers, container_details)
				continue
			}

		} else {
			if container_details.Container.Config.Image != required_service.Container.Config.Image {
				remaining_containers = append(remaining_containers, container_details)
				continue
			}
		}

		if required_service.Container.Config.Hostname != "" && container_details.Container.Config.Hostname != required_service.Container.Config.Hostname {
			remaining_containers = append(remaining_containers, container_details)
			continue
		}
		if container_details.Container.Name != required_service.Name {
			remaining_containers = append(remaining_containers, container_details)
			continue
		}

		/* TBD
		if required_container.DockerOptions["Env"] != nil {
			envs := required_container.DockerOptions["Env"].([]interface{})
			for _, env := range envs {
				fmt.Println("env ", env.(string), "\n")
			}
		}
		*/

		if found {
			found_containers = append(found_containers, container_details)
			//fmt.Println("Found matching!", container_details)
		} else {
			remaining_containers = append(remaining_containers, container_details)
		}
	}

	return found_containers, remaining_containers
}

func ConvergeContainers(conf MachineConfiguration, client *docker.Client) {
	var opts docker.ListContainersOptions
	var ready_for_launch []ServiceConfiguration
	opts.All = true
	existing_containers_info, err := client.ListContainers(opts)
	if err != nil {
		return
	}

	var existing_containers []ContainerDetails
	for _, container_info := range existing_containers_info {
		//fmt.Printf("Got container: %+v\n", container_info)
		container := ContainerDetails{APIContainers: container_info}
		container.Container, err = client.InspectContainer(container.ID)
		if err != nil {
			panic(err)
		}

		// For some reason the container name has / prefix (eg. "/comet"). Strip it out
		if container.Container.Name[0] == '/' {
			container.Container.Name = container.Container.Name[1:]
		}

		existing_containers = append(existing_containers, container)
	}

	var matching_containers []ContainerDetails
	for _, required_bound_service := range conf.Services {
		required_service := required_bound_service.GetConfig()
		if required_service.Container == nil {
			continue
		}

		matching_containers, existing_containers = FindMatchingContainers(existing_containers, required_service)

		if len(matching_containers) > 1 {
			fmt.Println("Weird! Found more than one container matching specs: ", matching_containers)
		}

		if len(matching_containers) == 0 {
			fmt.Println("No containers found matching ", required_service, ". Marking for launch...")
			ready_for_launch = append(ready_for_launch, required_service)
		}

		if len(matching_containers) == 1 {
			if !matching_containers[0].Container.State.Running {
				fmt.Println("Found one matching container and it's not running! Removing it so we can start it again", matching_containers[0])
				err = client.RemoveContainer(docker.RemoveContainerOptions{matching_containers[0].Container.ID, true, true})
				if err != nil {
					panic(err)
				}

			}
		}
	}

	//	fmt.Println("Remaining running containers: ", len(existing_containers))
	var imageRegexp = regexp.MustCompile("(.+):")
	for _, container := range existing_containers {
		m := imageRegexp.FindStringSubmatch(container.Image)
		if len(m) == 0 {
			fmt.Printf("Invalid string submatch for container.Image %s\n", container.Image)
		} else {
			image := m[1]

			for _, authoritative_name := range conf.AuthoritativeNames {
				if authoritative_name == image {
					log.Info(LogEvent(ContainerLogEvent{"stop-and-remove", container.Container.Image, container.Container.Name}))

					fmt.Printf("Found container %+v which we are authoritative but its running. Going to stop it...\n", container)
					client.StopContainer(container.Container.ID, 10)
					err = client.RemoveContainer(docker.RemoveContainerOptions{container.Container.ID, true, true})
					if err != nil {
						panic(err)
					}
				}
			}
		}
	}

	for _, container := range ready_for_launch {
		LaunchContainer(container, client)
	}

}

func GetContainerImageNameWithRevision(serviceConfiguration ServiceConfiguration, revision string) string {
	var imageRegexp = regexp.MustCompile("(.+):")

	if revision == "" && serviceConfiguration.Revision != nil && serviceConfiguration.Revision.Revision != "" {
		revision = serviceConfiguration.Revision.Revision
	}

	if revision != "" {
		m := imageRegexp.FindStringSubmatch(serviceConfiguration.Container.Config.Image)
		return m[1] + ":" + revision

	}

	return serviceConfiguration.Container.Config.Image
}

func (c *ServiceConfiguration) GetRevision() string {
	var imageRegexp = regexp.MustCompile("(.+):(.+)")

	if c.Revision != nil && c.Revision.Revision != "" {
		return c.Revision.Revision
	} else {
		m := imageRegexp.FindStringSubmatch(c.Container.Config.Image)
		return m[2]
	}
}

func GetContainerImage(imageName string, client *docker.Client) (*docker.Image, error) {
	if client == nil {
		client = GetDockerClient()
	}

	// Check if we need to pull the image first
	image, err := client.InspectImage(imageName)
	if err != nil && err != docker.ErrNoSuchImage {
		return nil, err
	}

	return image, nil
}

type RepositoryTagResponse struct {
	LastUpdate int64 `json:"last_update"`
}

func VerifyContainerExistsInRepository(image_name string, overrided_revision string) (bool, int64, error) {
	// http://registry.applifier.info:5000/comet:ac937833f0af968be564230820a625c17f2e3ef1
	var imageRegexp = regexp.MustCompile("(.+)/(.+?):(.+)")
	m := imageRegexp.FindStringSubmatch(image_name)

	if overrided_revision != "" {
		m[3] = overrided_revision
	}

	// http://registry.applifier.info:5000/v1/repositories/comet/tags/ac937833f0af968be564230820a625c17f2e3ef1/json
	url := "http://" + m[1] + "/v1/repositories/" + m[2] + "/tags/" + m[3] + "/json"

	resp, err := http.Get(url)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	var data RepositoryTagResponse

	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		return false, 0, err
	}

	if data.LastUpdate == 0 {
		return false, 0, nil
	}
	return true, data.LastUpdate, nil
}

func LaunchContainer(serviceConfiguration ServiceConfiguration, client *docker.Client) {

	imageName := GetContainerImageNameWithRevision(serviceConfiguration, "")

	image, err := GetContainerImage(imageName, client)
	if err != nil {
		panic(err)
	}

	if image == nil {
		log.Info(LogEvent(ContainerLogEvent{"pulling", imageName, ""}))
		var pullImageOptions docker.PullImageOptions
		pullImageOptions.Registry = imageName[0:strings.Index(imageName, "/")]
		imagePlusTag := imageName[strings.Index(imageName, "/")+1:]
		pullImageOptions.Repository = pullImageOptions.Registry + "/" + imagePlusTag[0:strings.Index(imagePlusTag, ":")]
		pullImageOptions.Tag = imagePlusTag[strings.Index(imagePlusTag, ":")+1:]
		pullImageOptions.OutputStream = os.Stderr

		ret := client.PullImage(pullImageOptions, docker.AuthConfiguration{})
		fmt.Println("Ret:", ret)
	}

	var options docker.CreateContainerOptions
	options.Name = serviceConfiguration.Name
	var config docker.Config = serviceConfiguration.Container.Config
	config.Image = imageName
	options.Config = &config

	var addresses []string
	addresses, err = net.LookupHost("skydns.services.dev.docker")
	if err == nil {
		serviceConfiguration.Container.HostConfig.Dns = []string{addresses[0]}
		serviceConfiguration.Container.HostConfig.DnsSearch = []string{"services.dev.docker"}
	}

	// Check if we need to stop and remove the old container
	var existing_containers_info []docker.APIContainers
	existing_containers_info, err = client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		panic(err)
	}

	for _, container_info := range existing_containers_info {
		container := ContainerDetails{APIContainers: container_info}
		container.Container, err = client.InspectContainer(container.ID)
		if err != nil {
			fmt.Printf("error inspecting container. %+v\n", err)
			panic(err)
		}

		// For some reason the container name has / prefix (eg. "/comet"). Strip it out
		if container.Container.Name[0] == '/' {
			container.Container.Name = container.Container.Name[1:]
		}

		if container.Container.Name == serviceConfiguration.Name {
			fmt.Printf("Need to delete this old container container info: %+v\n", container_info)
			err = client.StopContainer(container.ID, 10)
			if err != nil {
				fmt.Printf("Could not stop container: %+v. Err: %+v\n", container_info, err)
			}
			err = client.RemoveContainer(docker.RemoveContainerOptions{container.ID, true, true})
			if err != nil {
				fmt.Printf("Could not remove container: %+v. Err: %+v\n", container_info, err)
			}
			break
		}
	}

	fmt.Println("Creating container", options)
	log.Info(LogEvent(ContainerLogEvent{"create-and-launch", imageName, serviceConfiguration.Name}))
	container, err := client.CreateContainer(options)
	if err != nil {
		panic(err)
	}

	err = client.StartContainer(container.ID, &serviceConfiguration.Container.HostConfig)
	if err != nil {
		log.Error("Could not start container")
		panic(err)
	}

}
