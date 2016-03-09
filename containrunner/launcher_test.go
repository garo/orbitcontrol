package containrunner

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetServiceConfigurationString(t *testing.T) {

	var str = GetServiceConfigurationString()
	assert.NotNil(t, str)
}

func TestGetContainerImageNameWithRevision(t *testing.T) {
	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Revision = new(ServiceRevision)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Revision.Revision = "asdfasdfasdf"
	revision := GetContainerImageNameWithRevision(required_service, "")

	assert.Equal(t, "registry.applifier.info:5000/comet:asdfasdfasdf", revision)

	required_service.Revision.Revision = ""
	revision = GetContainerImageNameWithRevision(required_service, "")
	assert.Equal(t, "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2", revision)

	required_service.Revision.Revision = "asdf"
	revision = GetContainerImageNameWithRevision(required_service, "foobar")
	assert.Equal(t, "registry.applifier.info:5000/comet:foobar", revision)

	required_service.Revision = nil
	revision = GetContainerImageNameWithRevision(required_service, "")
	assert.Equal(t, "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2", revision)

	required_service.Revision = nil
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet"
	revision = GetContainerImageNameWithRevision(required_service, "")
	assert.Equal(t, "registry.applifier.info:5000/comet:latest", revision)

	required_service.Revision = nil
	required_service.Container.Config.Image = "ubuntu"
	revision = GetContainerImageNameWithRevision(required_service, "")
	assert.Equal(t, "ubuntu:latest", revision)

	required_service.Revision = nil
	required_service.Container.Config.Image = "ubuntu:latest"
	revision = GetContainerImageNameWithRevision(required_service, "")
	assert.Equal(t, "ubuntu:latest", revision)

}

func TestConvergeContainers(t *testing.T) {

	client := GetDockerClient()

	var containrunner Containrunner
	conf, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	fmt.Printf("***** TestConvergeContainers\n")
	ConvergeContainers(conf.MachineConfigurations["testtag"], false, false, client)

}

func TestFindMatchingContainers_AllMatch(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 1, len(found_containers))
	assert.Equal(t, 0, len(remaining_containers))
}

func TestFindMatchingContaineres_Hostname_Mismatch(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com1"
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 0, len(found_containers))
	assert.Equal(t, 1, len(remaining_containers))
}

func TestFindMatchingContaineres_Name_Mismatch(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Name = "the-name1"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 0, len(found_containers))
	assert.Equal(t, 1, len(remaining_containers))
}

func TestFindMatchingContaineres_Env_Match(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"
	ec[0].Container.Config.Env = []string{"ENV=staging", "FOO=BAR"}

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Container.Config.Env = []string{"ENV=staging", "FOO=BAR"}
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 1, len(found_containers))
	assert.Equal(t, 0, len(remaining_containers))
}

func TestFindMatchingContaineres_Env_Match2(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"
	ec[0].Container.Config.Env = []string{"ENV=staging", "FOO=BAR", "JAVA_HOME=/opt/java"}

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Container.Config.Env = []string{"ENV=staging", "FOO=BAR"}
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 1, len(found_containers))
	assert.Equal(t, 0, len(remaining_containers))
}

func TestFindMatchingContaineres_Env_Match3(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"
	ec[0].Container.Config.Env = []string{"ENV=staging", "FOO=BAR", "JAVA_HOME=/opt/java"}

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 1, len(found_containers))
	assert.Equal(t, 0, len(remaining_containers))

}

func TestFindMatchingContaineres_Env_Mismatch(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"
	ec[0].Container.Config.Env = []string{"ENV=staging", "FOO=BAR"}

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Container.Config.Env = []string{"ENV=prod", "FOO=BAR"}
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 0, len(found_containers))
	assert.Equal(t, 1, len(remaining_containers))
}

func TestFindMatchingContaineres_Env_Mismatch2(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"
	ec[0].Container.Config.Env = []string{"FOO=BAR"}

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Container.Config.Env = []string{"FOO=BAR", "ENV=staging"}
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 0, len(found_containers))
	assert.Equal(t, 1, len(remaining_containers))
}

func TestFindMatchingContaineres_Env_Mismatch4(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Container.Config.Env = []string{"FOO=BAR"}
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 0, len(found_containers))
	assert.Equal(t, 1, len(remaining_containers))
}

func TestFindMatchingContaineres_Env_Mismatch5(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	ec[0].Container.Config.Hostname = "foo.bar.com"
	ec[0].Container.Name = "the-name"

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Container.Config.Hostname = "foo.bar.com"
	required_service.Container.Config.Env = []string{"ENV=staging", "FOO=BAR"}
	required_service.Name = "the-name"

	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 0, len(found_containers))
	assert.Equal(t, 1, len(remaining_containers))
}

func TestFindMatchingContaineres_Revision_Mismatch(t *testing.T) {
	var ec = make([]ContainerDetails, 1, 1)
	ec[0].Container = new(docker.Container)
	ec[0].Container.Config = new(docker.Config)
	ec[0].Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"

	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Revision = new(ServiceRevision)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Revision.Revision = "asdfasdfasdf"
	found_containers, remaining_containers := FindMatchingContainers(ec, required_service)

	assert.Equal(t, 0, len(found_containers))
	assert.Equal(t, 1, len(remaining_containers))

}
