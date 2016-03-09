package containrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
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

		// NOTE: this has bugs if the revision is "latest" on either side
		//fmt.Printf("image: %s, Config.Image: %s\n", container_details.Container.Config.Image, required_service.Container.Config.Image)

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

		if required_service.Container.Config.Env != nil || container_details.Container.Config.Env != nil {
			// Check first that all required envs are found in the suspect container
			for _, env1 := range required_service.Container.Config.Env {
				env1p := strings.Split(env1, "=")

				env_found := false

				for _, env2 := range container_details.Container.Config.Env {
					env2p := strings.Split(env2, "=")

					if env1p[0] == env2p[0] && env1p[1] == env2p[1] {
						env_found = true
						break
					}
				}
				if env_found == false {
					found = false
				}
			}

			// Then check the other way around: verify that all envs in the suspect container are found in the required container
			for _, env1 := range container_details.Container.Config.Env {
				env1p := strings.Split(env1, "=")

				key_found := false
				env_match := false

				for _, env2 := range required_service.Container.Config.Env {
					env2p := strings.Split(env2, "=")

					if env1p[0] == env2p[0] {
						key_found = true
						if env1p[1] == env2p[1] {
							env_match = true
						}
						break
					}
				}
				if key_found == true {

					if env_match == false {
						found = false
					} else {
					}
				}
			}

		}

		if found {
			found_containers = append(found_containers, container_details)
			//fmt.Println("Found matching!", container_details)
		} else {
			remaining_containers = append(remaining_containers, container_details)
			//fmt.Println("did not match!", container_details)

		}
	}

	return found_containers, remaining_containers
}

func ConvergeContainers(conf MachineConfiguration, preDelay bool, postDelay bool, client *docker.Client) error {
	var opts docker.ListContainersOptions
	var ready_for_launch []ServiceConfiguration
	opts.All = true
	existing_containers_info, err := client.ListContainers(opts)
	if err != nil {
		return nil // TODO: fix
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

		//log.Debug("required_bound_service: %+v", required_bound_service)
		if required_service.Container == nil {
			continue
		}

		matching_containers, existing_containers = FindMatchingContainers(existing_containers, required_service)

		if len(matching_containers) > 1 {
			log.Warning("Weird! Found more than one container matching specs: ", matching_containers)
		}

		if len(matching_containers) == 0 {
			log.Debug("No containers found matching ", required_service, ". Marking for launch...")
			ready_for_launch = append(ready_for_launch, required_service)
		}

		if len(matching_containers) == 1 {
			if !matching_containers[0].Container.State.Running {
				log.Debug("Found one matching container and it's not running! Removing it so we can start it again", matching_containers[0])
				err = client.RemoveContainer(docker.RemoveContainerOptions{matching_containers[0].Container.ID, true, true})
				if err != nil {
					log.Warning("Tried to delete container %s which was supposed to exists", matching_containers[0].Container.ID)
				}

			}
		}
	}

	//fmt.Println("Remaining running containers: ", len(existing_containers))
	var imageRegexp = regexp.MustCompile("(.+):")
	for _, container := range existing_containers {
		m := imageRegexp.FindStringSubmatch(container.Image)
		if len(m) >= 1 {
			image := m[1]

			for _, authoritative_name := range conf.AuthoritativeNames {
				if authoritative_name == image {
					//log.Info(LogEvent(ContainerLogEvent{"stop-and-remove", container.Container.Image, container.Container.Name}))

					log.Debug("Found container %s (%s) which we are authoritative but its running. Going to stop it...\n", container.APIContainers.ID, container.APIContainers.Image)
					client.StopContainer(container.Container.ID, 40)
					err = client.RemoveContainer(docker.RemoveContainerOptions{container.Container.ID, true, true})
					if err != nil {
						log.Panic(err)
					}
				}
			}
		}
	}

	var preserveImages []string = []string{}
	for _, container := range ready_for_launch {
		imageName := GetContainerImageNameWithRevision(container, "")
		preserveImages = append(preserveImages, imageName)
	}

	log.Debug("Preserving images %+v\n", preserveImages)

	err = CleanupOldAuthoritativeImages(conf.AuthoritativeNames, preserveImages, client)
	if err != nil {
		log.Warning("Error on cleaning up old images! %+v\n", err)
	}

	var somethingFailed error = nil
	for _, container := range ready_for_launch {
		imageName := GetContainerImageNameWithRevision(container, "")

		err = LaunchContainer(container.Name, imageName, container.Container, preDelay, postDelay, client)
		if err != nil {
			somethingFailed = err
		}
	}

	return somethingFailed
}

type Int64Slice []int64

func (a Int64Slice) Len() int           { return len(a) }
func (a Int64Slice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Int64Slice) Less(i, j int) bool { return a[i] < a[j] }

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

/**
 * Cleans up old revisions for all images in the authoritative_names list. Leaves two newest revision/tag
 * per image. preserve_names can be used to list images which must be preserved, for example images that we know
 * want to run in the near future.
 */
func CleanupOldAuthoritativeImages(authoritative_names []string, preserve_names []string, client *docker.Client) error {
	var imageRegexp = regexp.MustCompile("(.+):(.+)")

	opts := docker.ListImagesOptions{}

	storedImages, err := client.ListImages(opts)
	if err != nil {
		return err
	}

	// This maps <image name> to map of <image revision/tag> which then contains the APIImages object
	imagesForName := make(map[string]map[string]docker.APIImages)

	for _, image := range storedImages {
		for _, tag := range image.RepoTags {
			m := imageRegexp.FindStringSubmatch(tag)

			_, found := imagesForName[m[1]]
			if !found {
				imagesForName[m[1]] = make(map[string]docker.APIImages)
			}

			imagesForName[m[1]][m[2]] = image
		}
	}

	// Now iterate thru all interesting images
	for _, images := range imagesForName {
		//fmt.Printf("Name %s has %d images\n", image, len(images))

		// If an image has more than two different revisions/tags then we'll keep the two newest and erase the rest
		if len(images) > 2 {

			// We first build a map from image.Created to the actual APIImages
			createdMap := make(map[int64]docker.APIImages)

			// And store all the image.Create timestamps here
			var keys Int64Slice

			for _, image := range images {
				_, found := createdMap[image.Created]
				if !found {
					preserve := false
					for _, pn := range preserve_names {
						if stringInSlice(pn, image.RepoTags) {
							preserve = true
						}
					}
					if !preserve {
						createdMap[image.Created] = image
						keys = append(keys, image.Created)
					} else {
						log.Debug("Preserving image %+v\n", image)
					}
				}
			}

			// So we can sort the timestamps, oldest first
			sort.Sort(keys)

			// And then we start removing from the oldest and stop so that we leave two newest
			for i := 0; i < len(keys)-2; i++ {
				image := createdMap[keys[i]]
				log.Debug("Removing image %s (tags %+v) at timestamp %d\n", image.ID, image.RepoTags, keys[i])
				err = client.RemoveImage(image.ID)
				if err != nil {
					//log.Warning("Could not remove old image %+v, reason: %+v", image.ID, err)
				}
			}
		}
	}

	return nil
}

func GetContainerImageNameWithRevision(serviceConfiguration ServiceConfiguration, revision string) string {
	var imageRegexp = regexp.MustCompile("^(.+/.+?)(?::(.+?))?$")

	if revision == "" && serviceConfiguration.Revision != nil && serviceConfiguration.Revision.Revision != "" {
		revision = serviceConfiguration.Revision.Revision
	}

	m := imageRegexp.FindStringSubmatch(serviceConfiguration.Container.Config.Image)
	if len(m) == 0 {
		if revision != "" {
			return serviceConfiguration.Container.Config.Image + ":" + revision
		} else {
			if strings.Index(serviceConfiguration.Container.Config.Image, ":") != -1 {
				return serviceConfiguration.Container.Config.Image
			} else {
				return serviceConfiguration.Container.Config.Image + ":latest"
			}
		}
	} else {
		if revision != "" {
			return m[1] + ":" + revision
		} else if m[2] == "" {
			return serviceConfiguration.Container.Config.Image + ":latest"
		} else {
			return serviceConfiguration.Container.Config.Image
		}
	}

}

func (c *ServiceConfiguration) GetRevision() string {
	var imageRegexp = regexp.MustCompile("(.+):(.+)")

	if c.Revision != nil && c.Revision.Revision != "" {
		return c.Revision.Revision
	} else {
		m := imageRegexp.FindStringSubmatch(c.Container.Config.Image)
		if len(m) < 2 {
			log.Error("Error getting revision for %s\n", c.Container.Config.Image)
			return ""
		}
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

	if len(m) == 0 {
		return false, 0, errors.New("Invalid image name format. Maybe this is not a local registry? Global Docker registry is currently not supported")
	}

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

func LaunchContainer(name string, imageName string, container *ContainerConfiguration, preDelay bool, postDelay bool, client *docker.Client) error {

	image, err := GetContainerImage(imageName, client)
	if err != nil {
		panic(err)
	}

	if image == nil {
		for tries := 0; ; tries++ {
			if preDelay == true {
				delay := rand.Intn(40) + 1
				fmt.Printf("Sleeping %d seconds before pulling image %s\n", delay, imageName)
				time.Sleep(time.Second * time.Duration(delay))
			}
			//log.Info(LogEvent(ContainerLogEvent{"pulling", imageName, ""}))
			var pullImageOptions docker.PullImageOptions
			if strings.Index(imageName, "/") == -1 {
				pullImageOptions.Repository = imageName
			} else {
				fmt.Printf("Image name: %s\n", imageName)
				pullImageOptions.Registry = imageName[0:strings.Index(imageName, "/")]
				imagePlusTag := imageName[strings.Index(imageName, "/")+1:]
				pullImageOptions.Repository = pullImageOptions.Registry + "/" + imagePlusTag[0:strings.Index(imagePlusTag, ":")]
				pullImageOptions.Tag = imagePlusTag[strings.Index(imagePlusTag, ":")+1:]
			}

			pullImageOptions.OutputStream = os.Stderr

			err = client.PullImage(pullImageOptions, docker.AuthConfiguration{})
			if err != nil {
				if tries > 2 {
					fmt.Printf("Could not pull, too many tries. Aborting")
					return errors.New("Could not pull, too many tries")
				}
				fmt.Printf("Could not pull new image, possibly the registry is overloaded. Trying again soon. This was try %d\n%+v\n", tries, err)

				time.Sleep(time.Second * time.Duration(rand.Intn(60)+5))
			} else {
				break
			}

		}
	}

	var options docker.CreateContainerOptions
	options.Name = name
	var config docker.Config = container.Config
	config.Image = imageName
	options.Config = &config

	if postDelay {
		delay := rand.Intn(40) + 1
		fmt.Printf("Sleeping %d seconds before relaunching container %s\n", delay, imageName)
		time.Sleep(time.Second * time.Duration(delay))
	}

	DestroyContainer(name, client)

	log.Notice("Creating container %s", options.Name)
	//log.Info(LogEvent(ContainerLogEvent{"create-and-launch", imageName, name}))
	new_container, err := client.CreateContainer(options)
	if err != nil {
		fmt.Printf("Error on CreateContainer %+v: %+v", options, err)
		return err
	}

	err = client.StartContainer(new_container.ID, &container.HostConfig)
	if err != nil {
		log.Error("Could not start container")
		fmt.Printf("Error on StartContainer ID %s: %+v", new_container.ID, err)
	}

	return nil
}

func DestroyContainer(name string, client *docker.Client) error {

	// Check if we need to stop and remove the old container
	var existing_containers_info []docker.APIContainers
	existing_containers_info, err := client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		fmt.Printf("Error listing containers: %+v\n", err)
		return err
	}

	for _, container_info := range existing_containers_info {
		container := ContainerDetails{APIContainers: container_info}
		container.Container, err = client.InspectContainer(container.ID)
		if err != nil {
			fmt.Printf("error inspecting container. %+v\n", err)
		}

		// For some reason the container name has / prefix (eg. "/comet"). Strip it out
		if container.Container.Name[0] == '/' {
			container.Container.Name = container.Container.Name[1:]
		}

		if container.Container.Name == name {
			log.Notice("Stopping container %+v", container_info)
			err = client.StopContainer(container.ID, 10)
			if err != nil {
				log.Warning("Could not stop container: %+v. Err: %+v\n", container_info, err)
			}
			err = client.RemoveContainer(docker.RemoveContainerOptions{container.ID, true, true})
			if err != nil {
				log.Warning("Could not remove container: %+v. Err: %+v\n", container_info, err)
			}
			break
		}
	}

	return nil
}
