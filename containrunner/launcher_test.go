package containrunner

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	. "gopkg.in/check.v1"
	"testing"
)

//import "github.com/stretchr/testify/mock"

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type MySuite struct {
	client *docker.Client
}

var _ = Suite(&MySuite{})

func (s *MySuite) SetUpTest(c *C) {
	s.client = GetDockerClient()
	c.Assert(s.client, Not(IsNil))
}

func (s *MySuite) TestGetServiceConfigurationString(c *C) {

	var str = GetServiceConfigurationString()
	c.Assert(str, Not(IsNil))
}

func (s *MySuite) TestConvergeContainers(c *C) {

	var containrunner Containrunner
	conf, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	fmt.Printf("***** TestConvergeContainers\n")
	ConvergeContainers(conf.MachineConfigurations["testtag"], false, s.client)

}

func (s *MySuite) TestFindMatchingContainers_AllMatch(c *C) {
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

	c.Assert(len(found_containers), Equals, 1)
	c.Assert(len(remaining_containers), Equals, 0)
}

func (s *MySuite) TestFindMatchingContaineres_Hostname_Mismatch(c *C) {
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

	c.Assert(len(found_containers), Equals, 0)
	c.Assert(len(remaining_containers), Equals, 1)
}

func (s *MySuite) TestFindMatchingContaineres_Name_Mismatch(c *C) {
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

	c.Assert(len(found_containers), Equals, 0)
	c.Assert(len(remaining_containers), Equals, 1)
}

func (s *MySuite) TestFindMatchingContaineres_Env_Match(c *C) {
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

	c.Assert(len(found_containers), Equals, 1)
	c.Assert(len(remaining_containers), Equals, 0)
}

func (s *MySuite) TestFindMatchingContaineres_Env_Match2(c *C) {
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

	c.Assert(len(found_containers), Equals, 1)
	c.Assert(len(remaining_containers), Equals, 0)
}

func (s *MySuite) TestFindMatchingContaineres_Env_Match3(c *C) {
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

	c.Assert(len(found_containers), Equals, 1)
	c.Assert(len(remaining_containers), Equals, 0)
}

func (s *MySuite) TestFindMatchingContaineres_Env_Mismatch(c *C) {
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

	c.Assert(len(found_containers), Equals, 0)
	c.Assert(len(remaining_containers), Equals, 1)
}

func (s *MySuite) TestFindMatchingContaineres_Env_Mismatch2(c *C) {
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

	c.Assert(len(found_containers), Equals, 0)
	c.Assert(len(remaining_containers), Equals, 1)
}

func (s *MySuite) TestFindMatchingContaineres_Env_Mismatch4(c *C) {
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

	c.Assert(len(found_containers), Equals, 0)
	c.Assert(len(remaining_containers), Equals, 1)
}

func (s *MySuite) TestFindMatchingContaineres_Env_Mismatch5(c *C) {
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

	c.Assert(len(found_containers), Equals, 0)
	c.Assert(len(remaining_containers), Equals, 1)
}
func (s *MySuite) TestFindMatchingContaineres_Revision_Mismatch(c *C) {
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

	c.Assert(len(found_containers), Equals, 0)
	c.Assert(len(remaining_containers), Equals, 1)
}

func (s *MySuite) TestGetContainerImageNameWithRevision(c *C) {
	var required_service ServiceConfiguration
	required_service.Container = new(ContainerConfiguration)
	required_service.Revision = new(ServiceRevision)
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
	required_service.Revision.Revision = "asdfasdfasdf"
	revision := GetContainerImageNameWithRevision(required_service, "")

	c.Assert(revision, Equals, "registry.applifier.info:5000/comet:asdfasdfasdf")

	required_service.Revision.Revision = ""
	revision = GetContainerImageNameWithRevision(required_service, "")
	c.Assert(revision, Equals, "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2")

	required_service.Revision.Revision = "asdf"
	revision = GetContainerImageNameWithRevision(required_service, "foobar")
	c.Assert(revision, Equals, "registry.applifier.info:5000/comet:foobar")

	required_service.Revision = nil
	revision = GetContainerImageNameWithRevision(required_service, "")
	c.Assert(revision, Equals, "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2")

	required_service.Revision = nil
	required_service.Container.Config.Image = "registry.applifier.info:5000/comet"
	revision = GetContainerImageNameWithRevision(required_service, "")
	c.Assert(revision, Equals, "registry.applifier.info:5000/comet:latest")

	required_service.Revision = nil
	required_service.Container.Config.Image = "ubuntu"
	revision = GetContainerImageNameWithRevision(required_service, "")
	c.Assert(revision, Equals, "ubuntu:latest")

}
