package containrunner

import "testing"

//import "fmt"
import . "gopkg.in/check.v1"
import "github.com/fsouza/go-dockerclient"

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

func (s *MySuite) TestGetConfiguration(c *C) {
	var str = `
{
	"services": {
		"comet": {
			"Name": "comet",
			"Container" : {
				"HostConfig" : {
					"Binds": [
						"/tmp:/data"
					],
					"NetworkMode" : "host"				
				},
				"Config": {
					"Env": [
						"NODE_ENV=production"
					],
					"AttachStderr": false,
					"AttachStdin": false,
					"AttachStdout": false,
					"OpenStdin": false,
					"Hostname": "comet",
					"Image": "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
				}
			},
			"checks" : [
				{
					"type" : "http",
					"url" : "http://localhost:3500/check"
				}
			]
		}
	},
	"authoritative_names": [
		"registry.applifier.info:5000/comet"
	]
}
`

	var conf = GetConfiguration(str)

	var service ServiceConfiguration = conf.Services["comet"]
	c.Assert(service.Name, Equals, "comet")
	c.Assert(service.Container.HostConfig.NetworkMode, Equals, "host")
	c.Assert(service.Container.HostConfig.Binds[0], Equals, "/tmp:/data")

	c.Assert(service.Container.Config.Env[0], Equals, "NODE_ENV=production")
	c.Assert(service.Container.Config.Image, Equals, "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2")

	//ßc.Assert(conf.DockerOptions["Env"][0], Equals, "NODE_ENV=production")

	c.Assert(service.Container.Config.AttachStderr, Equals, false)
	c.Assert(service.Container.Config.AttachStdin, Equals, false)
	c.Assert(service.Container.Config.AttachStdout, Equals, false)
	c.Assert(service.Container.Config.Hostname, Equals, "comet")

	check := service.Checks[0]
	c.Assert(check.Type, Equals, "http")
	c.Assert(check.Url, Equals, "http://localhost:3500/check")

}

func (s *MySuite) TestConvergeContainers(c *C) {
	var str = `
{
	"containers": {
		"comet": {
			"Name": "comet",
			"Container" : {
				"HostConfig" : {
					"Binds": [
						"/tmp:/data"
					],
					"NetworkMode" : "host"				
				},
				"Config": {
					"Env": [
						"NODE_ENV=production"
					],
					"AttachStderr": false,
					"AttachStdin": false,
					"AttachStdout": false,
					"OpenStdin": false,
					"Hostname": "comet",
					"Image": "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2"
				}
			},
			"checks" : [
				{
					"type" : "http",
					"url" : "http://localhost:3500/check"
				}
			]
		}
	},
	"authoritative_names": [
		"registry.applifier.info:5000/comet"
	]
}
`
	var conf = GetConfiguration(str)

	ConvergeContainers(conf, s.client)
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
