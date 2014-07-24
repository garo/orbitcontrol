package containrunner

import . "gopkg.in/check.v1"

import "github.com/coreos/go-etcd/etcd"

//import "strings"

type ConfigBridgeSuite struct {
	etcd *etcd.Client
}

var _ = Suite(&ConfigBridgeSuite{})

func (s *ConfigBridgeSuite) SetUpTest(c *C) {
	s.etcd = etcd.NewClient([]string{"http://etcd:4001"})
	s.etcd.DeleteDir("/test/")

}

func (s *ConfigBridgeSuite) TestLoadOrbitConfigurationFromFiles(c *C) {
	var ct Containrunner

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	c.Assert(err, IsNil)

	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration, Not(IsNil))

	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.GlobalSection, Equals,
		"global\n"+
			"stats socket /var/run/haproxy/admin.sock level admin user haproxy group haproxy\n"+
			"\tlog 127.0.0.1\tlocal2 info\n"+
			"\tmaxconn 16000\n"+
			" \tulimit-n 40000\n"+
			"\tuser haproxy\t\n"+
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

	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Endpoints, Not(IsNil))
	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Endpoints["comet"], Not(IsNil))
	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Endpoints["comet"].Name, Equals, "comet")
	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Endpoints["comet"].Config.PerServer, Equals, "check inter 2000")

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

	res, err := s.etcd.Get("/test/machineconfigurations/tags/testtag/haproxy_endpoints/comet", true, true)
	c.Assert(err, IsNil)

	c.Assert(res.Node.Value, Equals, "{\"Name\":\"comet\",\"Config\":{\"PerServer\":\"check inter 2000\",\"ListenAddress\":\"0.0.0.0:80\",\"Listen\":\"mode http\\nbalance leastconn\\nstats uri /haproxy-status66\",\"Backend\":\"\"}}")

	res, err = s.etcd.Get("/test/services/comet/config", true, true)
	c.Assert(err, IsNil)

	c.Assert(res.Node.Value, Equals, "{\"Name\":\"comet\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\"}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"ContainerIDFile\":\"\",\"LxcConf\":null,\"Privileged\":false,\"PortBindings\":null,\"Links\":null,\"PublishAllPorts\":false,\"Dns\":null,\"DnsSearch\":null,\"VolumesFrom\":null,\"NetworkMode\":\"host\"},\"Config\":{\"Hostname\":\"\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"NODE_ENV=production\"],\"Cmd\":null,\"Dns\":null,\"Image\":\"registry.applifier.info:5000/comet:8fd079b54719d61b6feafbb8056b9ba09ade4760\",\"Volumes\":null,\"VolumesFrom\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false}}}")

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
	]
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

	scribedEndpoint :=
		`{
			"Name":"scribed",
			"Config" : {
				"PerServer" : "",
				"Backend" : "mode http\n"
			}
		}
`

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/haproxy_endpoints/scribed", scribedEndpoint, 10)
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

	c.Assert(configuration.HAProxyConfiguration.Endpoints["scribed"].Name, Equals, "scribed")
	c.Assert(configuration.HAProxyConfiguration.Endpoints["scribed"].Config.PerServer, Equals, "")
	c.Assert(configuration.HAProxyConfiguration.Endpoints["scribed"].Config.Backend, Equals, "mode http\n")

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

func (s *ConfigBridgeSuite) TestGetHAProxyEndpoints(c *C) {
	var mc MachineConfiguration
	mc.HAProxyConfiguration = NewHAProxyConfiguration()

	var endpoint = new(HAProxyEndpoint)
	mc.HAProxyConfiguration.Endpoints["testService2"] = endpoint
	endpoint.Name = "testService2"

	_, err := s.etcd.Set("/test/services/testService2/endpoints/10.1.2.3:1234", "foobar", 10)
	if err != nil {
		panic(err)
	}

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	err = containrunner.GetHAProxyEndpoints(s.etcd, &mc)
	c.Assert(err, IsNil)

	c.Assert(mc.HAProxyConfiguration.Endpoints["testService2"].BackendServers["10.1.2.3:1234"], Equals, "foobar")

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
