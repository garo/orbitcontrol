package containrunner

import (
	"encoding/json"
	"fmt"
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
	s.etcd.RawDelete("/test/", true, true)

}

func (s *ConfigBridgeSuite) TestLoadOrbitConfigurationFromFiles(c *C) {
	var ct Containrunner

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	c.Assert(err, IsNil)

	c.Assert(orbitConfiguration.GlobalOrbitProperties.AMQPUrl, Equals, "amqp://guest:guest@localhost:5672/")

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

	fmt.Printf("TEST: %+v\n", orbitConfiguration.MachineConfigurations["testtag"].Services["dashboards"])

	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].Services["ubuntu"], Not(IsNil))
	c.Assert(orbitConfiguration.MachineConfigurations["testtag"].AuthoritativeNames[0], Equals, "ubuntu")

	c.Assert(orbitConfiguration.Services["ubuntu"].Name, Equals, "ubuntu")
	c.Assert(orbitConfiguration.Services["ubuntu"].EndpointPort, Equals, 3500)
	c.Assert(orbitConfiguration.Services["ubuntu"].Checks[0].Type, Equals, "http")

}

func (s *ConfigBridgeSuite) TestUploadOrbitConfigurationToEtcd(c *C) {
	var ct Containrunner
	ct.EtcdBasePath = "/test"

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	c.Assert(err, IsNil)

	s.etcd.DeleteDir("/test/")

	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, s.etcd)
	c.Assert(err, IsNil)

	res, err := s.etcd.Get("/test/globalproperties", true, true)
	c.Assert(err, IsNil)
	var gop GlobalOrbitProperties
	err = json.Unmarshal([]byte(res.Node.Value), &gop)
	c.Assert(gop.AMQPUrl, Equals, `amqp://guest:guest@localhost:5672/`)

	res, err = s.etcd.Get("/test/machineconfigurations/tags/testtag/haproxy_files/500.http", true, true)
	c.Assert(err, IsNil)
	c.Assert(res.Node.Value, Equals, `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`)

	res, err = s.etcd.Get("/test/machineconfigurations/tags/testtag/authoritative_names", true, true)
	c.Assert(err, IsNil)
	c.Assert(res.Node.Value, Equals, "[\"ubuntu\"]")

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

	res, err = s.etcd.Get("/test/services/ubuntu/config", true, true)
	c.Assert(err, IsNil)
	c.Assert(res.Node.Value, Equals, "{\"Name\":\"ubuntu\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HttpHost\":\"\",\"Username\":\"\",\"Password\":\"\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\",\"ConnectTimeout\":0,\"ResponseTimeout\":0,\"Delay\":0}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"ContainerIDFile\":\"\",\"LxcConf\":null,\"Privileged\":false,\"PortBindings\":null,\"Links\":null,\"PublishAllPorts\":false,\"Dns\":null,\"DnsSearch\":null,\"VolumesFrom\":null,\"NetworkMode\":\"host\"},\"Config\":{\"Hostname\":\"\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"NODE_ENV=vagrant\"],\"Cmd\":null,\"Dns\":null,\"Image\":\"ubuntu\",\"Volumes\":null,\"VolumesFrom\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false}},\"Revision\":null,\"SourceControl\":{\"Origin\":\"github.com/Applifier/ubuntu\",\"OAuthToken\":\"\",\"CIUrl\":\"\"},\"Attributes\":{}}")

	res, err = s.etcd.Get("/test/machineconfigurations/tags/testtag/services/ubuntu", true, true)
	c.Assert(err, IsNil)

	expected := "{\"Name\":\"\",\"EndpointPort\":0,\"Checks\":null,\"Container\":{\"HostConfig\":{\"Binds\":null,\"ContainerIDFile\":\"\",\"LxcConf\":null,\"Privileged\":false,\"PortBindings\":null,\"Links\":null,\"PublishAllPorts\":false,\"Dns\":null,\"DnsSearch\":null,\"VolumesFrom\":null,\"NetworkMode\":\"\"},\"Config\":{\"Hostname\":\"ubuntu-test\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"NODE_ENV=staging\"],\"Cmd\":null,\"Dns\":null,\"Image\":\"ubuntu\",\"Volumes\":null,\"VolumesFrom\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false}},\"Revision\":null,\"SourceControl\":null,\"Attributes\":null}"
	c.Assert(res.Node.Value, Equals, expected)

}

func (s *ConfigBridgeSuite) TestGetGlobalOrbitProperties(c *C) {
	var ct Containrunner
	ct.EtcdBasePath = "/test"

	gop := GlobalOrbitProperties{}
	gop.AMQPUrl = "amqp://guest:guest@localhost:5672/"
	bytes, _ := json.Marshal(gop)
	_, err := s.etcd.Set("/test/globalproperties", string(bytes), 10)

	gop, err = ct.GetGlobalOrbitProperties(s.etcd)
	c.Assert(err, IsNil)
	c.Assert(gop.AMQPUrl, Equals, `amqp://guest:guest@localhost:5672/`)

}

func (s *ConfigBridgeSuite) TestUploadOrbitConfigurationToEtcdWhichRemovesAService(c *C) {
	var ct Containrunner
	ct.EtcdBasePath = "/test"

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	c.Assert(err, IsNil)

	s.etcd.DeleteDir("/test/")

	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, s.etcd)
	c.Assert(err, IsNil)

	// Verify that this service exists before we try to delete it in the second step
	res, err := s.etcd.Get("/test/services/ubuntu/config", true, true)
	c.Assert(err, IsNil)
	c.Assert(res.Node.Value, Equals, "{\"Name\":\"ubuntu\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HttpHost\":\"\",\"Username\":\"\",\"Password\":\"\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\",\"ConnectTimeout\":0,\"ResponseTimeout\":0,\"Delay\":0}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"ContainerIDFile\":\"\",\"LxcConf\":null,\"Privileged\":false,\"PortBindings\":null,\"Links\":null,\"PublishAllPorts\":false,\"Dns\":null,\"DnsSearch\":null,\"VolumesFrom\":null,\"NetworkMode\":\"host\"},\"Config\":{\"Hostname\":\"\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"NODE_ENV=vagrant\"],\"Cmd\":null,\"Dns\":null,\"Image\":\"ubuntu\",\"Volumes\":null,\"VolumesFrom\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false}},\"Revision\":null,\"SourceControl\":{\"Origin\":\"github.com/Applifier/ubuntu\",\"OAuthToken\":\"\",\"CIUrl\":\"\"},\"Attributes\":{}}")

	orbitConfiguration, err = ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	c.Assert(err, IsNil)

	// Delete the service from the orbitConfiguration...
	delete(orbitConfiguration.MachineConfigurations["testtag"].Services, "ubuntu")

	// ...so it should be deleted by the following UploadOrbitConfigurationToEtcd call
	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, s.etcd)
	c.Assert(err, IsNil)

	res, err = s.etcd.Get("/test/machineconfigurations/tags/testtag/services/ubuntu", true, true)
	fmt.Printf("removed service error: %+v\n", err)
	fmt.Printf("removed service res: %+v\n", res)

	c.Assert(err, Not(IsNil))

}

func (s *ConfigBridgeSuite) TestMergeServiceConfig(c *C) {
	defaults := new(ServiceConfiguration)
	overwrite := new(ServiceConfiguration)

	json.Unmarshal([]byte(`
{
	"Name": "ubuntu",
	"EndpointPort" : 80,
	"Container" : {
		"HostConfig" : {
			"Binds": [
				"/tmp:/data"
			],
			"NetworkMode" : "host"
		},
		"Config": {
			"Env": [
				"FOO=BAR",
				"NODE_ENV=production"
			],
			"AttachStderr": false,
			"AttachStdin": false,
			"AttachStdout": false,
			"OpenStdin": false,
			"Hostname": "ubuntu",
			"Image": "ubuntu"
		}
	},
	"checks" : [
		{
			"type" : "http",
			"url" : "http://localhost:3500/check"
		}
	],
	"Attributes" : {

	}
}
`), defaults)

	json.Unmarshal([]byte(`
{
	"EndpointPort" : 8002,
	"Container" : {
		"Config": {
			"Env": [
				"NODE_ENV=staging"
			],
			"Image":"ubuntu",
			"Hostname": "ubuntu-test"
		}
	},
	"checks" : [
		{
			"type" : "http",
			"url" : "http://localhost:8002/check"
		}
	],
	"Attributes" : {
		"foo" : "bar"
	}
}
`), overwrite)

	merged := MergeServiceConfig(*defaults, *overwrite)

	c.Assert(merged.Name, Equals, "ubuntu")
	c.Assert(merged.EndpointPort, Equals, 8002)
	c.Assert(merged.Container.HostConfig.Binds[0], Equals, "/tmp:/data")
	c.Assert(merged.Container.Config.Env[0], Equals, "FOO=BAR")
	c.Assert(merged.Container.Config.Env[1], Equals, "NODE_ENV=staging")
	c.Assert(merged.Container.Config.Image, Equals, "ubuntu")
	c.Assert(merged.Container.Config.Hostname, Equals, "ubuntu-test")
	c.Assert(merged.Checks[0].Type, Equals, "http")
	c.Assert(merged.Checks[0].Url, Equals, "http://localhost:8002/check")
	c.Assert(merged.Attributes["foo"], Equals, "bar")

}

func (s *ConfigBridgeSuite) TestGetMachineConfigurationByTags(c *C) {

	_, err := s.etcd.CreateDir("/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")
	}

	var ubuntu = `
{
	"Name": "ubuntu",
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
			"Hostname": "ubuntu",
			"Image": "ubuntu"
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
	_, err = s.etcd.Set("/services/ubuntu/config", ubuntu, 10)
	if err != nil {
		panic(err)
	}

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = s.etcd.Set("/services/ubuntu/revision", revision, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/services/ubuntu", ``, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/authoritative_names", `["ubuntu"]`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/haproxy_config", "foobar", 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.CreateDir("/machineconfigurations/tags/testtag/haproxy_files", 10)
	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/haproxy_files/hello.txt", "hello", 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/certs/test.pem", "----TEST-----", 10)
	if err != nil {
		panic(err)
	}

	tags := []string{"testtag"}
	var containrunner Containrunner

	configuration, err := containrunner.GetMachineConfigurationByTags(s.etcd, tags, "")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Name, Equals, "ubuntu")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.HostConfig.NetworkMode, Equals, "host")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.Config.AttachStderr, Equals, false)
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.Config.Hostname, Equals, "ubuntu")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.Config.Image, Equals, "ubuntu")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Checks[0].Type, Equals, "http")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Checks[0].Url, Equals, "http://localhost:3500/check")

	c.Assert(configuration.HAProxyConfiguration.Template, Equals, "foobar")
	c.Assert(configuration.HAProxyConfiguration.Certs["test.pem"], Equals, "----TEST-----")
	c.Assert(configuration.HAProxyConfiguration.Files["hello.txt"], Equals, "hello")

	c.Assert(configuration.Services["ubuntu"].GetConfig().Revision.Revision, Equals, "asdf")

	_, _ = s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")

}

func (s *ConfigBridgeSuite) TestGetMachineConfigurationByTagsWithOverwrittenParameters(c *C) {

	_, err := s.etcd.CreateDir("/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")
	}

	var ubuntu = `
{
	"Name": "ubuntu",
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
			"Hostname": "ubuntu",
			"Image": "ubuntu"
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
	_, err = s.etcd.Set("/services/ubuntu/config", ubuntu, 10)
	if err != nil {
		panic(err)
	}

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = s.etcd.Set("/services/ubuntu/revision", revision, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/services/ubuntu", `
{
	"Container" : {
		"Config": {
			"Env": [
				"NODE_ENV=staging"
			]
		}
	}
}`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/authoritative_names", `["ubuntu"]`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/haproxy_config", "foobar", 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.CreateDir("/machineconfigurations/tags/testtag/haproxy_files", 10)
	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/haproxy_files/hello.txt", "hello", 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set("/machineconfigurations/tags/testtag/certs/test.pem", "----TEST-----", 10)
	if err != nil {
		panic(err)
	}

	tags := []string{"testtag"}
	var containrunner Containrunner
	configuration, err := containrunner.GetMachineConfigurationByTags(s.etcd, tags, "")

	c.Assert(configuration.Services["ubuntu"].GetConfig().Name, Equals, "ubuntu")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.HostConfig.NetworkMode, Equals, "host")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.Config.AttachStderr, Equals, false)
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.Config.Env[0], Equals, "NODE_ENV=staging")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.Config.Hostname, Equals, "ubuntu")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Container.Config.Image, Equals, "ubuntu")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Checks[0].Type, Equals, "http")
	c.Assert(configuration.Services["ubuntu"].GetConfig().Checks[0].Url, Equals, "http://localhost:3500/check")

	c.Assert(configuration.HAProxyConfiguration.Template, Equals, "foobar")
	c.Assert(configuration.HAProxyConfiguration.Certs["test.pem"], Equals, "----TEST-----")
	c.Assert(configuration.HAProxyConfiguration.Files["hello.txt"], Equals, "hello")

	c.Assert(configuration.Services["ubuntu"].GetConfig().Revision.Revision, Equals, "asdf")

	_, _ = s.etcd.DeleteDir("/machineconfigurations/tags/testtag/")

}
func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherServiceOk(c *C) {

	crep := ConfigResultEtcdPublisher{5, "/test", nil, s.etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)

	res, err := s.etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	if err != nil {
		panic(err)
	}
	c.Assert(res.Node.Value, Equals, "null")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	c.Assert(res.Node.TTL, Equals, int64(5))

	_, _ = s.etcd.DeleteDir("/test/services/testService/endpoints/10.1.2.3:1234")
}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherServiceWithEndpointInfo(c *C) {

	crep := ConfigResultEtcdPublisher{5, "/test", nil, s.etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, &EndpointInfo{
		Revision: "asdf",
	})

	res, err := s.etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	if err != nil {
		panic(err)
	}
	endpointInfo := EndpointInfo{}
	err = json.Unmarshal([]byte(res.Node.Value), &endpointInfo)
	c.Assert(err, IsNil)
	c.Assert(endpointInfo.Revision, Equals, "asdf")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	c.Assert(res.Node.TTL, Equals, int64(5))

	_, _ = s.etcd.DeleteDir("/test/services/testService/endpoints/10.1.2.3:1234")
}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherServiceNotOk(c *C) {

	crep := ConfigResultEtcdPublisher{5, "/test", nil, s.etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", false, nil)

	_, err := s.etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	c.Assert(err, Not(IsNil))
}

func (s *ConfigBridgeSuite) TestConfigResultEtcdPublisherWithPreviousExistingValue(c *C) {

	crep := ConfigResultEtcdPublisher{5, "/test", nil, s.etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)
	_, _ = s.etcd.DeleteDir("/test/services/testService/endpoints/0.1.2.3:1234")

}

func (s *ConfigBridgeSuite) TestGetEndpointsForService(c *C) {

	_, err := s.etcd.Set("/test/services/testService2/endpoints/10.1.2.3:1234", "{\"Revision\":\"foobar\"}", 10)
	if err != nil {
		panic(err)
	}

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	containrunner.EtcdEndpoints = TestingEtcdEndpoints
	endpoints, err := containrunner.GetEndpointsForService("testService2")
	c.Assert(err, IsNil)

	c.Assert(endpoints["10.1.2.3:1234"].Revision, Equals, "foobar")

}

func (s *ConfigBridgeSuite) TestGetAllEndpoints(c *C) {

	_, err := s.etcd.Set("/test/services/testService2/endpoints/10.1.2.3:1234", "{\"Revision\":\"foo\"}", 10)
	c.Assert(err, IsNil)
	_, err = s.etcd.Set("/test/services/testService2/endpoints/10.1.2.4:1234", "{\"Revision\":\"bar\"}", 10)
	c.Assert(err, IsNil)
	_, err = s.etcd.Set("/test/services/testService1/endpoints/10.1.2.4:1000", "{\"Revision\":\"kissa\"}", 10)
	c.Assert(err, IsNil)

	serviceEndpoints, err := GetAllServiceEndpoints(TestingEtcdEndpoints, "/test")
	c.Assert(err, IsNil)

	c.Assert(serviceEndpoints["testService1"]["10.1.2.4:1000"].Revision, Equals, "kissa")
	c.Assert(serviceEndpoints["testService2"]["10.1.2.3:1234"].Revision, Equals, "foo")
	c.Assert(serviceEndpoints["testService2"]["10.1.2.4:1234"].Revision, Equals, "bar")

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

func (s *ConfigBridgeSuite) TestGetServiceByName(c *C) {
	s.etcd.Delete("/test/services/myservice/revision", true)
	s.etcd.Set("/test/services/myservice/revision", `
{
	"Revision" : "asdfasdf"
}`, 10)

	s.etcd.Delete("/test/services/myservice/machines", true)
	s.etcd.Set("/test/services/myservice/machines/10.0.0.1", `
{
	"Revision" : "newrevision"
}`, 10)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	serviceConfiguration, err := containrunner.GetServiceByName("myservice", s.etcd, "10.0.0.1")
	c.Assert(err, IsNil)
	c.Assert(serviceConfiguration.Revision.Revision, Equals, "newrevision")

	s.etcd.Delete("/test/services/myservice/revision", true)
	s.etcd.Delete("/test/services/myservice/machines/10.0.0.1", true)
	s.etcd.Delete("/test/services/myservice/machines", true)

}

func (s *ConfigBridgeSuite) TestSetServiceRevision(c *C) {

	s.etcd.Delete("/test/services/myservice/revision", true)
	s.etcd.Set("/test/services/myservice/machines/10.0.0.1", `
{
	"Revision" : "newrevision"
}`, 10)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"

	serviceRevision := ServiceRevision{
		Revision: "asdf",
	}
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

	res, err = s.etcd.Get("/test/services/myservice/machines/10.0.0.1", false, false)
	c.Assert(err, Not(IsNil))

	s.etcd.Delete("/test/services/myservice/revision", true)
}

func (s *ConfigBridgeSuite) TestSetServiceRevisionForMachine(c *C) {

	s.etcd.Delete("/test/services/myservice/machines", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"

	serviceRevision := ServiceRevision{
		Revision: "asdf",
	}
	var serviceRevision2 ServiceRevision

	err := containrunner.SetServiceRevisionForMachine("myservice", serviceRevision, "10.0.0.2", s.etcd)
	c.Assert(err, IsNil)

	res, err := s.etcd.Get("/test/services/myservice/machines/10.0.0.2", false, false)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal([]byte(res.Node.Value), &serviceRevision2)
	c.Assert(err, IsNil)

	c.Assert(serviceRevision2.Revision, Equals, "asdf")

	s.etcd.Delete("/test/services/myservice/machines", true)
}
