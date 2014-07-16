package containrunner

import . "gopkg.in/check.v1"

//import "fmt"
import "github.com/coreos/go-etcd/etcd"

//import "strings"

type ConfigBridgeSuite struct {
	etcd *etcd.Client
}

var _ = Suite(&ConfigBridgeSuite{})

func (s *ConfigBridgeSuite) SetUpTest(c *C) {
	s.etcd = etcd.NewClient([]string{"http://etcd:4001"})
}

func (s *ConfigBridgeSuite) TestGetMachineConfiguration(c *C) {

	_, err := s.etcd.CreateDir("/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")
	}

	var comet = `
{
			"Name": "comet",
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
			},
			"checks" : [
				{
					"type" : "http",
					"url" : "http://localhost:3500/check"
				}
			]
		}
`
	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/services/comet", comet, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/authoritative_names", `["registry.applifier.info:5000/comet"]`, 10)
	if err != nil {
		panic(err)
	}

	tags := []string{"testtag"}
	configuration, err := GetMachineConfigurationByTags(s.etcd, tags)

	c.Assert(configuration.Containers["comet"].Name, Equals, "comet")
	c.Assert(configuration.Containers["comet"].HostConfig.NetworkMode, Equals, "host")
	c.Assert(configuration.Containers["comet"].Config.AttachStderr, Equals, false)
	c.Assert(configuration.Containers["comet"].Config.Hostname, Equals, "comet")
	c.Assert(configuration.Containers["comet"].Config.Image, Equals, "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2")
	c.Assert(configuration.Containers["comet"].Checks[0].Type, Equals, "http")
	c.Assert(configuration.Containers["comet"].Checks[0].Url, Equals, "http://localhost:3500/check")

	_, _ = s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")

}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherServiceOk(c *C) {

	crep := ConfigResultEtcdPublisher{s.etcd, 5}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true)

	res, err := s.etcd.Get("/services/testService/10.1.2.3:1234", false, false)
	if err != nil {
		panic(err)
	}
	c.Assert(res.Node.Value, Equals, "{}")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	c.Assert(res.Node.TTL, Equals, int64(5))

	_, _ = s.etcd.DeleteDir("/services/testService/10.1.2.3:1234")
}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherServiceNotOk(c *C) {

	crep := ConfigResultEtcdPublisher{s.etcd, 5}
	crep.PublishServiceState("testService", "10.1.2.3:1234", false)

	_, err := s.etcd.Get("/services/testService/10.1.2.3:1234", false, false)
	c.Assert(err, Not(IsNil))
}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherWithPreviousExistingValue(c *C) {

	crep := ConfigResultEtcdPublisher{s.etcd, 5}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true)
	crep.PublishServiceState("testService", "10.1.2.3:1234", true)
	_, _ = s.etcd.DeleteDir("/services/testService/10.1.2.3:1234")

}
