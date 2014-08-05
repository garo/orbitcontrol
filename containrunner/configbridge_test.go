package containrunner

import (
	"encoding/json"
	"github.com/coreos/go-etcd/etcd"
	. "gopkg.in/check.v1"
)

type ConfigBridgeSuite struct {
	etcd *etcd.Client
}

var _ = Suite(&ConfigBridgeSuite{})

var TestingEtcdEndpoints []string = []string{"http://etcd:4001"}

func (s *ConfigBridgeSuite) SetUpTest(c *C) {
	s.etcd = etcd.NewClient(TestingEtcdEndpoints)
	s.etcd.DeleteDir("/test/")

}

func (s *ConfigBridgeSuite) TestLoadOrbitConfigurationFromFiles(c *C) {
	var ct Containrunner

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	c.Assert(err, IsNil)

	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration, Not(IsNil))

	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Template, Equals,
		"global\n"+
			"stats socket /var/run/haproxy/admin.sock level admin user haproxy group haproxy\n"+
			"\tlog 127.0.0.1\tlocal2 info\n"+
			"\tmaxconn 16000\n"+
			" \tulimit-n 40000\n"+
			"\tuser haproxy\n"+
			"\tgroup haproxy\n"+
			"\tdaemon\n"+
			"\tquiet\n"+
			"\tpidfile /var/run/haproxy.pid\n"+
			"\n"+
			"defaults\n"+
			"\tlog\tglobal\n"+
			"\tmode http\n"+
			"\toption httplog\n"+
			"\toption dontlognull\n"+
			"\tretries 3\n"+
			"\toption redispatch\n"+
			"\tmaxconn\t8000\n"+
			"\tcontimeout 5000\n"+
			"\tclitimeout 60000\n"+
			"\tsrvtimeout 60000\n"+
			"\n")

	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Files["500.http"], Equals,
		`HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`)

	c.Assert(orbitConfiguration.Services["comet"].Name, Equals, "comet")
	c.Assert(orbitConfiguration.Services["comet"].EndpointPort, Equals, 3500)
	c.Assert(orbitConfiguration.Services["comet"].Checks[0].Type, Equals, "http")

}

func (s *ConfigBridgeSuite) TestUploadOrbitConfigurationToEtcd(c *C) {
	var ct Containrunner
	ct.EtcdBasePath = "/test"

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	c.Assert(err, IsNil)

	s.etcd.DeleteDir("/test/")

	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, s.etcd)
	c.Assert(err, IsNil)

	res, err := s.etcd.Get("/test/machineconfigurations/tags/testtag/haproxy_files/500.http", true, true)
	c.Assert(err, IsNil)
	c.Assert(res.Node.Value, Equals, `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`)

	res, err = s.etcd.Get("/test/machineconfigurations/tags/testtag/haproxy_config", true, true)
	c.Assert(err, IsNil)

	c.Assert(res.Node.Value, Equals,
		"global\n"+
			"stats socket /var/run/haproxy/admin.sock level admin user haproxy group haproxy\n"+
			"\tlog 127.0.0.1\tlocal2 info\n"+
			"\tmaxconn 16000\n"+
			" \tulimit-n 40000\n"+
			"\tuser haproxy\n"+
			"\tgroup haproxy\n"+
			"\tdaemon\n"+
			"\tquiet\n"+
			"\tpidfile /var/run/haproxy.pid\n"+
			"\n"+
			"defaults\n"+
			"\tlog\tglobal\n"+
			"\tmode http\n"+
			"\toption httplog\n"+
			"\toption dontlognull\n"+
			"\tretries 3\n"+
			"\toption redispatch\n"+
			"\tmaxconn\t8000\n"+
			"\tcontimeout 5000\n"+
			"\tclitimeout 60000\n"+
			"\tsrvtimeout 60000\n"+
			"\n")

	res, err = s.etcd.Get("/test/services/comet/config", true, true)
	c.Assert(err, IsNil)

	c.Assert(res.Node.Value, Equals, "{\"Name\":\"comet\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\"}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"ContainerIDFile\":\"\",\"LxcConf\":null,\"Privileged\":false,\"PortBindings\":null,\"Links\":null,\"PublishAllPorts\":false,\"Dns\":null,\"DnsSearch\":null,\"VolumesFrom\":null,\"NetworkMode\":\"host\"},\"Config\":{\"Hostname\":\"\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"NODE_ENV=production\"],\"Cmd\":null,\"Dns\":null,\"Image\":\"registry.applifier.info:5000/comet:8fd079b54719d61b6feafbb8056b9ba09ade4760\",\"Volumes\":null,\"VolumesFrom\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false}},\"Revision\":null}")

}

func (s *ConfigBridgeSuite) TestGetMachineConfiguration(c *C) {

	_, err := s.etcd.CreateDir("/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")
	}

	var comet = `
{
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
	],
	"Revision" : {
		"Revision" : "asdf"
	}
}
`
	_, err = s.etcd.Set("/services/comet/config", comet, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/services/comet", ``, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/authoritative_names", `["registry.applifier.info:5000/comet"]`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/haproxy_config", "foobar", 10)
	if err != nil {
		panic(err)
	}

	tags := []string{"testtag"}
	var containrunner Containrunner
	configuration, err := containrunner.GetMachineConfigurationByTags(s.etcd, tags)

	c.Assert(configuration.Services["comet"].Name, Equals, "comet")
	c.Assert(configuration.Services["comet"].Container.HostConfig.NetworkMode, Equals, "host")
	c.Assert(configuration.Services["comet"].Container.Config.AttachStderr, Equals, false)
	c.Assert(configuration.Services["comet"].Container.Config.Hostname, Equals, "comet")
	c.Assert(configuration.Services["comet"].Container.Config.Image, Equals, "registry.applifier.info:5000/comet:874559764c3d841f3c45cf3ecdb6ecfa3eb19dd2")
	c.Assert(configuration.Services["comet"].Checks[0].Type, Equals, "http")
	c.Assert(configuration.Services["comet"].Checks[0].Url, Equals, "http://localhost:3500/check")

	c.Assert(configuration.HAProxyConfiguration.Template, Equals, "foobar")

	c.Assert(configuration.Services["comet"].Revision.Revision, Equals, "asdf")

	_, _ = s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")

}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherServiceOk(c *C) {

	crep := ConfigResultEtcdPublisher{5, "/test", nil, s.etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true)

	res, err := s.etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	if err != nil {
		panic(err)
	}
	c.Assert(res.Node.Value, Equals, "{}")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	c.Assert(res.Node.TTL, Equals, int64(5))

	_, _ = s.etcd.DeleteDir("/test/services/testService/endpoints/10.1.2.3:1234")
}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherServiceNotOk(c *C) {

	crep := ConfigResultEtcdPublisher{5, "/test", nil, s.etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", false)

	_, err := s.etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	c.Assert(err, Not(IsNil))
}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherWithPreviousExistingValue(c *C) {

	crep := ConfigResultEtcdPublisher{5, "/test", nil, s.etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true)
	crep.PublishServiceState("testService", "10.1.2.3:1234", true)
	_, _ = s.etcd.DeleteDir("/test/services/testService/endpoints/0.1.2.3:1234")

}

func (s *ConfigBridgeSuite) TestGetHAProxyEndpointsForService(c *C) {

	_, err := s.etcd.Set("/test/services/testService2/endpoints/10.1.2.3:1234", "foobar", 10)
	if err != nil {
		panic(err)
	}

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	containrunner.EtcdEndpoints = TestingEtcdEndpoints
	endpoints, err := containrunner.GetHAProxyEndpointsForService("testService2")
	c.Assert(err, IsNil)

	c.Assert(endpoints["10.1.2.3:1234"], Equals, "foobar")

}

func (s *ConfigBridgeSuite) TestGetAllServices(c *C) {

	_, err := s.etcd.Set("/test/services/testService2/config", `{
"Name" : "testService2",
"EndpointPort" : 1025
}`, 10)
	if err != nil {
		panic(err)
	}

	var containrunner Containrunner
	var services map[string]ServiceConfiguration
	containrunner.EtcdBasePath = "/test"
	services, err = containrunner.GetAllServices(s.etcd)
	c.Assert(err, IsNil)

	c.Assert(services["testService2"].Name, Equals, "testService2")
	c.Assert(services["testService2"].EndpointPort, Equals, 1025)

}

func (s *ConfigBridgeSuite) TestTagServiceToTag(c *C) {

	_, _ = s.etcd.Delete("/test/machineconfigurations/tags/testtag/services/myservice", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	err := containrunner.TagServiceToTag("myservice", "testtag", s.etcd)
	c.Assert(err, IsNil)

	res, err := s.etcd.Get("/test/machineconfigurations/tags/testtag/services/myservice", false, false)
	if err != nil {
		panic(err)
	}
	c.Assert(res.Node.Value, Equals, "{}")

	_, _ = s.etcd.Delete("/test/machineconfigurations/tags/testtag/services/myservice", true)

}

func (s *ConfigBridgeSuite) TestGetServiceRevision(c *C) {

	s.etcd.Delete("/test/services/myservice/revision", true)
	s.etcd.Set("/test/services/myservice/revision", `
{
	"Revision" : "asdfasdf"
}`, 10)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	serviceRevision, err := containrunner.GetServiceRevision("myservice", s.etcd)
	c.Assert(err, IsNil)
	c.Assert(serviceRevision.Revision, Equals, "asdfasdf")

	s.etcd.Delete("/test/services/myservice/revision", true)

}

func (s *ConfigBridgeSuite) TestSetServiceRevision(c *C) {

	s.etcd.Delete("/test/services/myservice/revision", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"

	serviceRevision := ServiceRevision{"asdf"}
	var serviceRevision2 ServiceRevision

	err := containrunner.SetServiceRevision("myservice", serviceRevision, s.etcd)
	c.Assert(err, IsNil)

	res, err := s.etcd.Get("/test/services/myservice/revision", false, false)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal([]byte(res.Node.Value), &serviceRevision2)
	c.Assert(err, IsNil)

	c.Assert(serviceRevision2.Revision, Equals, "asdf")

	s.etcd.Delete("/test/services/myservice/revision", true)

}
