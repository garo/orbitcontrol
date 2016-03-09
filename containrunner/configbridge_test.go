package containrunner

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/stretchr/testify/assert"
	"testing"
)

var TestingEtcdEndpoints []string = []string{"http://etcd:4001"}

func TestLoadOrbitConfigurationFromFiles(t *testing.T) {
	var ct Containrunner

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	assert.Nil(t, err)

	assert.Equal(t, "amqp://guest:guest@localhost:5672/", orbitConfiguration.GlobalOrbitProperties.AMQPUrl)

	assert.NotNil(t, orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration)

	assert.Equal(t,
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
			"\n", orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Template)

	assert.Equal(t,
		`HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`, orbitConfiguration.MachineConfigurations["testtag"].HAProxyConfiguration.Files["500.http"])

	assert.NotNil(t, orbitConfiguration.MachineConfigurations["testtag"].Services["ubuntu"])
	assert.Equal(t, "ubuntu", orbitConfiguration.MachineConfigurations["testtag"].AuthoritativeNames[0])

	assert.Equal(t, "ubuntu", orbitConfiguration.Services["ubuntu"].Name)
	assert.Equal(t, 3500, orbitConfiguration.Services["ubuntu"].EndpointPort)
	assert.Equal(t, "http", orbitConfiguration.Services["ubuntu"].Checks[0].Type)

}

func TestUploadOrbitConfigurationToEtcd(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	assert.Nil(t, err)

	etcd.DeleteDir("/test/")

	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, etcd)
	assert.Nil(t, err)

	res, err := etcd.Get("/test/globalproperties", true, true)
	assert.Nil(t, err)
	var gop GlobalOrbitProperties
	err = json.Unmarshal([]byte(res.Node.Value), &gop)
	assert.Equal(t, `amqp://guest:guest@localhost:5672/`, gop.AMQPUrl)

	res, err = etcd.Get("/test/machineconfigurations/tags/testtag/haproxy_files/500.http", true, true)
	assert.Nil(t, err)
	assert.Equal(t, `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`, res.Node.Value)

	res, err = etcd.Get("/test/machineconfigurations/tags/testtag/authoritative_names", true, true)
	assert.Nil(t, err)
	assert.Equal(t, "[\"ubuntu\"]", res.Node.Value)

	res, err = etcd.Get("/test/machineconfigurations/tags/testtag/haproxy_config", true, true)
	assert.Nil(t, err)

	assert.Equal(t,
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
			"\n", res.Node.Value)

	res, err = etcd.Get("/test/services/ubuntu/config", true, true)
	assert.Nil(t, err)
	assert.Equal(t, res.Node.Value, "{\"Name\":\"ubuntu\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HttpHost\":\"\",\"Username\":\"\",\"Password\":\"\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\",\"ConnectTimeout\":0,\"ResponseTimeout\":0,\"Delay\":0}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"NetworkMode\":\"host\",\"RestartPolicy\":{},\"LogConfig\":{}},\"Config\":{\"Env\":[\"NODE_ENV=vagrant\"],\"Image\":\"ubuntu\"}},\"Revision\":null,\"SourceControl\":{\"Origin\":\"github.com/Applifier/ubuntu\",\"OAuthToken\":\"\",\"CIUrl\":\"\"},\"Attributes\":{}}")

	res, err = etcd.Get("/test/machineconfigurations/tags/testtag/services/ubuntu", true, true)
	assert.Nil(t, err)

	expected := "{\"Name\":\"\",\"EndpointPort\":0,\"Checks\":null,\"Container\":{\"HostConfig\":{\"RestartPolicy\":{},\"LogConfig\":{}},\"Config\":{\"Hostname\":\"ubuntu-test\",\"Env\":[\"NODE_ENV=staging\"],\"Image\":\"ubuntu\"}},\"Revision\":null,\"SourceControl\":null,\"Attributes\":null}"
	assert.Equal(t, expected, res.Node.Value)

}

func TestGetGlobalOrbitProperties(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	gop := GlobalOrbitProperties{}
	gop.AMQPUrl = "amqp://guest:guest@localhost:5672/"
	bytes, _ := json.Marshal(gop)
	_, err := etcd.Set("/test/globalproperties", string(bytes), 10)

	gop, err = ct.GetGlobalOrbitProperties(etcd)
	assert.Nil(t, err)
	assert.Equal(t, `amqp://guest:guest@localhost:5672/`, gop.AMQPUrl)

}

func TestGetGlobalOrbitPropertiesWithAvailabilityZones(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	gop := NewGlobalOrbitProperties()
	gop.AMQPUrl = "amqp://guest:guest@localhost:5672/"
	gop.AvailabilityZones["us-east-1a"] = []string{"10.0.0.0/24", "10.0.1.0/24"}
	bytes, _ := json.Marshal(gop)
	_, err := etcd.Set("/test/globalproperties", string(bytes), 10)

	gop2, err := ct.GetGlobalOrbitProperties(etcd)
	assert.Nil(t, err)
	assert.Equal(t, `amqp://guest:guest@localhost:5672/`, gop2.AMQPUrl)
	assert.Equal(t, `10.0.0.0/24`, gop2.AvailabilityZones["us-east-1a"][0])
	assert.Equal(t, `10.0.1.0/24`, gop2.AvailabilityZones["us-east-1a"][1])

}

func TestUploadOrbitConfigurationToEtcdWhichRemovesAService(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	assert.Nil(t, err)

	etcd.DeleteDir("/test/")

	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, etcd)
	assert.Nil(t, err)

	// Verify that this service exists before we try to delete it in the second step
	res, err := etcd.Get("/test/services/ubuntu/config", true, true)
	assert.Nil(t, err)
	assert.Equal(t, "{\"Name\":\"ubuntu\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HttpHost\":\"\",\"Username\":\"\",\"Password\":\"\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\",\"ConnectTimeout\":0,\"ResponseTimeout\":0,\"Delay\":0}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"NetworkMode\":\"host\",\"RestartPolicy\":{},\"LogConfig\":{}},\"Config\":{\"Env\":[\"NODE_ENV=vagrant\"],\"Image\":\"ubuntu\"}},\"Revision\":null,\"SourceControl\":{\"Origin\":\"github.com/Applifier/ubuntu\",\"OAuthToken\":\"\",\"CIUrl\":\"\"},\"Attributes\":{}}", res.Node.Value)

	orbitConfiguration, err = ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	assert.Nil(t, err)

	// Delete the service from the orbitConfiguration...
	delete(orbitConfiguration.MachineConfigurations["testtag"].Services, "ubuntu")

	// ...so it should be deleted by the following UploadOrbitConfigurationToEtcd call
	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, etcd)
	assert.Nil(t, err)

	res, err = etcd.Get("/test/machineconfigurations/tags/testtag/services/ubuntu", true, true)
	fmt.Printf("removed service error: %+v\n", err)
	fmt.Printf("removed service res: %+v\n", res)

	assert.NotNil(t, err)

}

func TestMergeServiceConfig(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

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
				"NODE_ENV=production",
				"LDAP=dc=foo,dc=bar"
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

	assert.Equal(t, merged.Name, "ubuntu")
	assert.Equal(t, merged.EndpointPort, 8002)
	assert.Equal(t, merged.Container.HostConfig.Binds[0], "/tmp:/data")
	assert.Equal(t, merged.Container.Config.Env[0], "FOO=BAR")
	assert.Equal(t, merged.Container.Config.Env[2], "NODE_ENV=staging")
	assert.Equal(t, merged.Container.Config.Env[1], "LDAP=dc=foo,dc=bar")
	assert.Equal(t, merged.Container.Config.Image, "ubuntu")
	assert.Equal(t, merged.Container.Config.Hostname, "ubuntu-test")
	assert.Equal(t, merged.Checks[0].Type, "http")
	assert.Equal(t, merged.Checks[0].Url, "http://localhost:8002/check")
	assert.Equal(t, merged.Attributes["foo"], "bar")

}

func TestGetMachineConfigurationByTags(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	_, err := etcd.CreateDir("/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		etcd.DeleteDir("/machineconfigurations/tags/testtag/")
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
	_, err = etcd.Set("/services/ubuntu/config", ubuntu, 10)
	assert.Nil(t, err)

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = etcd.Set("/services/ubuntu/revision", revision, 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/services/ubuntu", ``, 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/authoritative_names", `["ubuntu"]`, 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/haproxy_config", "foobar", 10)
	assert.Nil(t, err)

	_, err = etcd.CreateDir("/machineconfigurations/tags/testtag/haproxy_files", 10)
	_, err = etcd.Set("/machineconfigurations/tags/testtag/haproxy_files/hello.txt", "hello", 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/certs/test.pem", "----TEST-----", 10)
	assert.Nil(t, err)

	tags := []string{"testtag"}
	var containrunner Containrunner

	configuration, err := containrunner.GetMachineConfigurationByTags(etcd, tags, "")

	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Name, "ubuntu")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.HostConfig.NetworkMode, "host")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.Config.AttachStderr, false)
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.Config.Hostname, "ubuntu")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.Config.Image, "ubuntu")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Checks[0].Type, "http")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Checks[0].Url, "http://localhost:3500/check")

	assert.Equal(t, configuration.HAProxyConfiguration.Template, "foobar")
	assert.Equal(t, configuration.HAProxyConfiguration.Certs["test.pem"], "----TEST-----")
	assert.Equal(t, configuration.HAProxyConfiguration.Files["hello.txt"], "hello")

	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Revision.Revision, "asdf")

	_, _ = etcd.DeleteDir("/machineconfigurations/tags/testtag/")

}

func TestGetMachineConfigurationByTagsWithOverwrittenParameters(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	_, err := etcd.CreateDir("/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		etcd.DeleteDir("/machineconfigurations/tags/testtag/")
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
	_, err = etcd.Set("/services/ubuntu/config", ubuntu, 10)
	assert.Nil(t, err)

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = etcd.Set("/services/ubuntu/revision", revision, 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/services/ubuntu", `
{
	"Container" : {
		"Config": {
			"Env": [
				"NODE_ENV=staging"
			]
		}
	}
}`, 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/authoritative_names", `["ubuntu"]`, 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/haproxy_config", "foobar", 10)
	assert.Nil(t, err)

	_, err = etcd.CreateDir("/machineconfigurations/tags/testtag/haproxy_files", 10)
	_, err = etcd.Set("/machineconfigurations/tags/testtag/haproxy_files/hello.txt", "hello", 10)
	assert.Nil(t, err)

	_, err = etcd.Set("/machineconfigurations/tags/testtag/certs/test.pem", "----TEST-----", 10)
	assert.Nil(t, err)

	tags := []string{"testtag"}
	var containrunner Containrunner
	configuration, err := containrunner.GetMachineConfigurationByTags(etcd, tags, "")

	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Name, "ubuntu")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.HostConfig.NetworkMode, "host")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.Config.AttachStderr, false)
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.Config.Env[0], "NODE_ENV=staging")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.Config.Hostname, "ubuntu")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Container.Config.Image, "ubuntu")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Checks[0].Type, "http")
	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Checks[0].Url, "http://localhost:3500/check")

	assert.Equal(t, configuration.HAProxyConfiguration.Template, "foobar")
	assert.Equal(t, configuration.HAProxyConfiguration.Certs["test.pem"], "----TEST-----")
	assert.Equal(t, configuration.HAProxyConfiguration.Files["hello.txt"], "hello")

	assert.Equal(t, configuration.Services["ubuntu"].GetConfig().Revision.Revision, "asdf")

	_, _ = etcd.DeleteDir("/machineconfigurations/tags/testtag/")

}
func TestConfigResultEtcdPublisherServiceOk(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)

	res, err := etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	assert.Nil(t, err)
	assert.Equal(t, res.Node.Value, "null")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	assert.Equal(t, res.Node.TTL, int64(5))

	_, _ = etcd.DeleteDir("/test/services/testService/endpoints/10.1.2.3:1234")
}

func TestConfigResultEtcdPublisherServiceWithEndpointInfo(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, &EndpointInfo{
		Revision: "asdf",
	})

	res, err := etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	assert.Nil(t, err)

	endpointInfo := EndpointInfo{}
	err = json.Unmarshal([]byte(res.Node.Value), &endpointInfo)
	assert.Nil(t, err)
	assert.Equal(t, endpointInfo.Revision, "asdf")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	assert.Equal(t, res.Node.TTL, int64(5))

	_, _ = etcd.DeleteDir("/test/services/testService/endpoints/10.1.2.3:1234")
}

func TestConfigResultEtcdPublisherServiceNotOk(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", false, nil)

	_, err := etcd.Get("/test/services/testService/endpoints/10.1.2.3:1234", false, false)
	assert.NotNil(t, err)
}

func TestConfigResultEtcdPublisherWithPreviousExistingValue(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcd}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)
	_, _ = etcd.DeleteDir("/test/services/testService/endpoints/0.1.2.3:1234")

}

func TestGetEndpointsForService(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	_, err := etcd.Set("/test/services/testService2/endpoints/10.1.2.3:1234", "{\"Revision\":\"foobar\"}", 10)
	assert.Nil(t, err)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	containrunner.EtcdEndpoints = TestingEtcdEndpoints
	endpoints, err := containrunner.GetEndpointsForService("testService2")
	assert.Nil(t, err)

	assert.Equal(t, endpoints["10.1.2.3:1234"].Revision, "foobar")

}

func TestGetAllEndpoints(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	_, err := etcd.Set("/test/services/testService2/endpoints/10.1.2.3:1234", "{\"Revision\":\"foo\"}", 10)
	assert.Nil(t, err)
	_, err = etcd.Set("/test/services/testService2/endpoints/10.1.2.4:1234", "{\"Revision\":\"bar\"}", 10)
	assert.Nil(t, err)
	_, err = etcd.Set("/test/services/testService1/endpoints/10.1.2.4:1000", "{\"Revision\":\"kissa\"}", 10)
	assert.Nil(t, err)

	serviceEndpoints, err := GetAllServiceEndpoints(TestingEtcdEndpoints, "/test")
	assert.Nil(t, err)

	assert.Equal(t, serviceEndpoints["testService1"]["10.1.2.4:1000"].Revision, "kissa")
	assert.Equal(t, serviceEndpoints["testService2"]["10.1.2.3:1234"].Revision, "foo")
	assert.Equal(t, serviceEndpoints["testService2"]["10.1.2.4:1234"].Revision, "bar")

}

func TestGetAllServices(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	_, err := etcd.Set("/test/services/testService2/config", `{
"Name" : "testService2",
"EndpointPort" : 1025
}`, 10)
	assert.Nil(t, err)

	var containrunner Containrunner
	var services map[string]ServiceConfiguration
	containrunner.EtcdBasePath = "/test"
	services, err = containrunner.GetAllServices(etcd)
	assert.Nil(t, err)

	assert.Equal(t, services["testService2"].Name, "testService2")
	assert.Equal(t, services["testService2"].EndpointPort, 1025)

}

func TestTagServiceToTag(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	_, _ = etcd.Delete("/test/machineconfigurations/tags/testtag/services/myservice", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	err := containrunner.TagServiceToTag("myservice", "testtag", etcd)
	assert.Nil(t, err)

	res, err := etcd.Get("/test/machineconfigurations/tags/testtag/services/myservice", false, false)
	assert.Nil(t, err)
	assert.Equal(t, res.Node.Value, "{}")

	_, _ = etcd.Delete("/test/machineconfigurations/tags/testtag/services/myservice", true)

}

func TestGetServiceRevision(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	etcd.Delete("/test/services/myservice/revision", true)
	etcd.Set("/test/services/myservice/revision", `
{
	"Revision" : "asdfasdf"
}`, 10)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	serviceRevision, err := containrunner.GetServiceRevision("myservice", etcd)
	assert.Nil(t, err)
	assert.Equal(t, serviceRevision.Revision, "asdfasdf")

	etcd.Delete("/test/services/myservice/revision", true)

}

func TestGetServiceByName(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	etcd.Delete("/test/services/myservice/revision", true)
	etcd.Set("/test/services/myservice/revision", `
{
	"Revision" : "asdfasdf"
}`, 10)

	etcd.Delete("/test/services/myservice/machines", true)
	etcd.Set("/test/services/myservice/machines/10.0.0.1", `
{
	"Revision" : "newrevision"
}`, 10)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	serviceConfiguration, err := containrunner.GetServiceByName("myservice", etcd, "10.0.0.1")
	assert.Nil(t, err)
	assert.Equal(t, serviceConfiguration.Revision.Revision, "newrevision")

	etcd.Delete("/test/services/myservice/revision", true)
	etcd.Delete("/test/services/myservice/machines/10.0.0.1", true)
	etcd.Delete("/test/services/myservice/machines", true)

}

func TestSetServiceRevision(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	etcd.Delete("/test/services/myservice/revision", true)
	etcd.Set("/test/services/myservice/machines/10.0.0.1", `
{
	"Revision" : "newrevision"
}`, 10)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"

	serviceRevision := ServiceRevision{
		Revision: "asdf",
	}
	var serviceRevision2 ServiceRevision

	err := containrunner.SetServiceRevision("myservice", serviceRevision, etcd)
	assert.Nil(t, err)

	res, err := etcd.Get("/test/services/myservice/revision", false, false)
	assert.Nil(t, err)

	err = json.Unmarshal([]byte(res.Node.Value), &serviceRevision2)
	assert.Nil(t, err)

	assert.Equal(t, serviceRevision2.Revision, "asdf")

	res, err = etcd.Get("/test/services/myservice/machines/10.0.0.1", false, false)
	assert.NotNil(t, err)

	etcd.Delete("/test/services/myservice/revision", true)
}

func TestSetServiceRevisionForMachine(t *testing.T) {
	etcd := etcd.NewClient(TestingEtcdEndpoints)
	defer etcd.Close()
	etcd.RawDelete("/test/", true, true)

	etcd.Delete("/test/services/myservice/machines", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"

	serviceRevision := ServiceRevision{
		Revision: "asdf",
	}
	var serviceRevision2 ServiceRevision

	err := containrunner.SetServiceRevisionForMachine("myservice", serviceRevision, "10.0.0.2", etcd)
	assert.Nil(t, err)

	res, err := etcd.Get("/test/services/myservice/machines/10.0.0.2", false, false)
	assert.Nil(t, err)

	err = json.Unmarshal([]byte(res.Node.Value), &serviceRevision2)
	assert.Nil(t, err)

	assert.Equal(t, serviceRevision2.Revision, "asdf")

	etcd.Delete("/test/services/myservice/machines", true)
}
