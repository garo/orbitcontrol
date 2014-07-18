package containrunner

import "encoding/json"
import "github.com/fsouza/go-dockerclient"
import "fmt"
import "strings"
import "os"
import "net"
import "regexp"

type ServiceCheck struct {
	Type             string
	Url              string
	DummyResult      bool
	ExpectHttpStatus string
	ExpectString     string
}

type ContainerConfiguration struct {
	HostConfig docker.HostConfig
	Config     docker.Config
}

type ServiceConfiguration struct {
	Name      string
	Checks    []ServiceCheck
	Container ContainerConfiguration
}

type MachineConfiguration struct {
	Services           map[string]ServiceConfiguration `json:"services"`
	HAProxyEndpoints   map[string]HAProxyEndpoint
	AuthoritativeNames []string `json:"authoritative_names"`
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

func GetConfiguration(str string) MachineConfiguration {
	var conf MachineConfiguration
	err := json.Unmarshal([]byte(str), &conf)
	if err != nil {
		panic(err)
	}

	return conf
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

	for _, container_details := range existing_containers {
		found := true
		if container_details.Container.Config.Image != required_service.Container.Config.Image {
			remaining_containers = append(remaining_containers, container_details)
			continue
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
		panic(err)
	}

	var existing_containers []ContainerDetails
	for _, container_info := range existing_containers_info {
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
	for _, required_service := range conf.Services {
		matching_containers, existing_containers = FindMatchingContainers(existing_containers, required_service)

		if len(matching_containers) > 1 {
			fmt.Println("Weird! Found more than one container matching specs: ", matching_containers)
		}

		if len(matching_containers) == 0 {
			fmt.Println("No containers found matching ", required_service, ". Marking for launch...")
			ready_for_launch = append(ready_for_launch, required_service)
		}

		if len(matching_containers) == 1 {
			if matching_containers[0].Container.State.Running {
				fmt.Println("Found one matching container and it's running")
			} else {
				fmt.Println("Found one matching container and it's not running!", matching_containers[0])
			}

		}
	}

	//	fmt.Println("Remaining running containers: ", len(existing_containers))
	var imageRegexp = regexp.MustCompile("(.+):")
	for _, container := range existing_containers {
		m := imageRegexp.FindStringSubmatch(container.Image)
		image := m[1]

		for _, authoritative_name := range conf.AuthoritativeNames {
			if authoritative_name == image {
				log.Info(LogEvent(ContainerLogEvent{"stop-and-remove", container.Container.Image, container.Container.Name}))

				//fmt.Printf("Found container %+v which we are authoritative but its running. Going to stop it...\n", container)
				client.StopContainer(container.Container.ID, 10)
				err = client.RemoveContainer(docker.RemoveContainerOptions{container.Container.ID, true, true})
				if err != nil {
					panic(err)
				}
			}
		}

	}

	for _, container := range ready_for_launch {
		LaunchContainer(container, client)
	}

}

func LaunchContainer(serviceConfiguration ServiceConfiguration, client *docker.Client) {

	// Check if we need to pull the image first
	image, err := client.InspectImage(serviceConfiguration.Container.Config.Image)
	if err != nil && err != docker.ErrNoSuchImage {
		panic(err)
	}

	if image == nil {
		log.Info(LogEvent(ContainerLogEvent{"pulling", serviceConfiguration.Container.Config.Image, ""}))
		var pullImageOptions docker.PullImageOptions
		pullImageOptions.Registry = serviceConfiguration.Container.Config.Image[0:strings.Index(serviceConfiguration.Container.Config.Image, "/")]
		imagePlusTag := serviceConfiguration.Container.Config.Image[strings.Index(serviceConfiguration.Container.Config.Image, "/")+1:]
		pullImageOptions.Repository = pullImageOptions.Registry + "/" + imagePlusTag[0:strings.Index(imagePlusTag, ":")]
		pullImageOptions.Tag = imagePlusTag[strings.Index(imagePlusTag, ":")+1:]
		pullImageOptions.OutputStream = os.Stderr

		ret := client.PullImage(pullImageOptions, docker.AuthConfiguration{})
		fmt.Println("Ret:", ret)
	}

	var options docker.CreateContainerOptions
	options.Name = serviceConfiguration.Name
	options.Config = &serviceConfiguration.Container.Config

	var addresses []string
	addresses, err = net.LookupHost("skydns.services.dev.docker")
	if err == nil {
		serviceConfiguration.Container.HostConfig.Dns = []string{addresses[0]}
		serviceConfiguration.Container.HostConfig.DnsSearch = []string{"services.dev.docker"}
	}

	//fmt.Println("Creating container", options)
	log.Info(LogEvent(ContainerLogEvent{"create-and-launch", serviceConfiguration.Container.Config.Image, serviceConfiguration.Name}))
	container, err := client.CreateContainer(options)
	if err != nil {
		panic(err)
	}

	err = client.StartContainer(container.ID, &serviceConfiguration.Container.HostConfig)
	if err != nil {
		panic(err)
	}

}
