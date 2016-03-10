package containrunner

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	etcd "github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var TestingEtcdEndpoints = []string{"http://localhost:4001"}

func GetTestingEtcdClient() etcd.KeysAPI {
	cfg := etcd.Config{
		Endpoints:               TestingEtcdEndpoints,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	client, err := etcd.New(cfg)

	if err != nil {
		panic(err)
	}

	kapi := etcd.NewKeysAPI(client)
	return kapi

}

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
	etcdClient := GetTestingEtcdClient()

	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	assert.Nil(t, err)

	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, etcdClient)
	if err != nil {
		fmt.Printf("UploadOrbitConfigurationToEtcd error: %+v\n", err)
	}

	assert.Nil(t, err)

	res, err := etcdClient.Get(context.Background(), "/test/globalproperties", &etcd.GetOptions{Recursive: true, Sort: true})
	assert.Nil(t, err)
	var gop GlobalOrbitProperties
	err = json.Unmarshal([]byte(res.Node.Value), &gop)
	assert.Equal(t, `amqp://guest:guest@localhost:5672/`, gop.AMQPUrl)

	res, err = etcdClient.Get(context.Background(), "/test/machineconfigurations/tags/testtag/haproxy_files/500.http", &etcd.GetOptions{Recursive: true, Sort: true})
	assert.Nil(t, err)
	assert.Equal(t, `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`, res.Node.Value)

	res, err = etcdClient.Get(context.Background(), "/test/machineconfigurations/tags/testtag/authoritative_names", &etcd.GetOptions{Recursive: true, Sort: true})
	assert.Nil(t, err)
	assert.Equal(t, "[\"ubuntu\"]", res.Node.Value)

	res, err = etcdClient.Get(context.Background(), "/test/machineconfigurations/tags/testtag/haproxy_config", &etcd.GetOptions{Recursive: true, Sort: true})
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

	res, err = etcdClient.Get(context.Background(), "/test/services/ubuntu/config", &etcd.GetOptions{Recursive: true, Sort: true})
	assert.Nil(t, err)
	assert.Equal(t, res.Node.Value, "{\"Name\":\"ubuntu\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HttpHost\":\"\",\"Username\":\"\",\"Password\":\"\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\",\"ConnectTimeout\":0,\"ResponseTimeout\":0,\"Delay\":0}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"NetworkMode\":\"host\",\"RestartPolicy\":{},\"LogConfig\":{}},\"Config\":{\"Env\":[\"NODE_ENV=vagrant\"],\"Image\":\"ubuntu\"}},\"Revision\":null,\"SourceControl\":{\"Origin\":\"github.com/Applifier/ubuntu\",\"OAuthToken\":\"\",\"CIUrl\":\"\"},\"Attributes\":{}}")

	res, err = etcdClient.Get(context.Background(), "/test/machineconfigurations/tags/testtag/services/ubuntu", &etcd.GetOptions{Recursive: true, Sort: true})
	assert.Nil(t, err)

	expected := "{\"Name\":\"\",\"EndpointPort\":0,\"Checks\":null,\"Container\":{\"HostConfig\":{\"RestartPolicy\":{},\"LogConfig\":{}},\"Config\":{\"Hostname\":\"ubuntu-test\",\"Env\":[\"NODE_ENV=staging\"],\"Image\":\"ubuntu\"}},\"Revision\":null,\"SourceControl\":null,\"Attributes\":null}"
	assert.Equal(t, expected, res.Node.Value)

}

func TestGetGlobalOrbitProperties(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	gop := GlobalOrbitProperties{}
	gop.AMQPUrl = "amqp://guest:guest@localhost:5672/"
	bytes, _ := json.Marshal(gop)
	_, err := etcdClient.Set(context.Background(), "/test/globalproperties", string(bytes), &etcd.SetOptions{TTL: 10})

	gop, err = ct.GetGlobalOrbitProperties(etcdClient)
	assert.Nil(t, err)
	assert.Equal(t, `amqp://guest:guest@localhost:5672/`, gop.AMQPUrl)

}

func TestGetGlobalOrbitPropertiesWithAvailabilityZones(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	gop := NewGlobalOrbitProperties()
	gop.AMQPUrl = "amqp://guest:guest@localhost:5672/"
	gop.AvailabilityZones["us-east-1a"] = []string{"10.0.0.0/24", "10.0.1.0/24"}
	bytes, _ := json.Marshal(gop)
	_, err := etcdClient.Set(context.Background(), "/test/globalproperties", string(bytes), &etcd.SetOptions{TTL: 10})

	gop2, err := ct.GetGlobalOrbitProperties(etcdClient)
	assert.Nil(t, err)
	assert.Equal(t, `amqp://guest:guest@localhost:5672/`, gop2.AMQPUrl)
	assert.Equal(t, `10.0.0.0/24`, gop2.AvailabilityZones["us-east-1a"][0])
	assert.Equal(t, `10.0.1.0/24`, gop2.AvailabilityZones["us-east-1a"][1])

}

func TestUploadOrbitConfigurationToEtcdWhichRemovesAService(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	var ct Containrunner
	ct.EtcdBasePath = "/test"

	orbitConfiguration, err := ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	assert.Nil(t, err)

	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, etcdClient)
	assert.Nil(t, err)

	// Verify that this service exists before we try to delete it in the second step
	res, err := etcdClient.Get(context.Background(), "/test/services/ubuntu/config", &etcd.GetOptions{Recursive: true, Sort: true})
	assert.Nil(t, err)
	assert.Equal(t, "{\"Name\":\"ubuntu\",\"EndpointPort\":3500,\"Checks\":[{\"Type\":\"http\",\"Url\":\"http://127.0.0.1:3500/check\",\"HttpHost\":\"\",\"Username\":\"\",\"Password\":\"\",\"HostPort\":\"\",\"DummyResult\":false,\"ExpectHttpStatus\":\"\",\"ExpectString\":\"\",\"ConnectTimeout\":0,\"ResponseTimeout\":0,\"Delay\":0}],\"Container\":{\"HostConfig\":{\"Binds\":[\"/tmp:/data\"],\"NetworkMode\":\"host\",\"RestartPolicy\":{},\"LogConfig\":{}},\"Config\":{\"Env\":[\"NODE_ENV=vagrant\"],\"Image\":\"ubuntu\"}},\"Revision\":null,\"SourceControl\":{\"Origin\":\"github.com/Applifier/ubuntu\",\"OAuthToken\":\"\",\"CIUrl\":\"\"},\"Attributes\":{}}", res.Node.Value)

	orbitConfiguration, err = ct.LoadOrbitConfigurationFromFiles("/Development/go/src/github.com/garo/orbitcontrol/testdata")
	assert.Nil(t, err)

	// Delete the service from the orbitConfiguration...
	delete(orbitConfiguration.MachineConfigurations["testtag"].Services, "ubuntu")

	// ...so it should be deleted by the following UploadOrbitConfigurationToEtcd call
	err = ct.UploadOrbitConfigurationToEtcd(orbitConfiguration, etcdClient)
	assert.Nil(t, err)

	res, err = etcdClient.Get(context.Background(), "/test/machineconfigurations/tags/testtag/services/ubuntu", &etcd.GetOptions{Recursive: true, Sort: true})
	fmt.Printf("removed service error: %+v\n", err)
	fmt.Printf("removed service res: %+v\n", res)

	assert.NotNil(t, err)

}

func TestMergeServiceConfig(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

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
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	_, err := etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/", "", &etcd.SetOptions{Dir: true})
	if err != nil {
		etcdClient.Delete(context.Background(), "/machineconfigurations/tags/testtag/", &etcd.DeleteOptions{Recursive: true})
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
	_, err = etcdClient.Set(context.Background(), "/services/ubuntu/config", ubuntu, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = etcdClient.Set(context.Background(), "/services/ubuntu/revision", revision, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/services/ubuntu", ``, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/authoritative_names", `["ubuntu"]`, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/haproxy_config", "foobar", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/haproxy_files", "", &etcd.SetOptions{Dir: true})
	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/haproxy_files/hello.txt", "hello", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/certs/test.pem", "----TEST-----", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	tags := []string{"testtag"}
	var containrunner Containrunner

	configuration, err := containrunner.GetMachineConfigurationByTags(etcdClient, tags, "")

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

	_, _ = etcdClient.Delete(context.Background(), "/machineconfigurations/tags/testtag/", &etcd.DeleteOptions{Recursive: true})

}

func TestGetMachineConfigurationByTagsWithOverwrittenParameters(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	_, err := etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/", "", &etcd.SetOptions{Dir: true})
	if err != nil {
		etcdClient.Delete(context.Background(), "/machineconfigurations/tags/testtag/", &etcd.DeleteOptions{Recursive: true})
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
	_, err = etcdClient.Set(context.Background(), "/services/ubuntu/config", ubuntu, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = etcdClient.Set(context.Background(), "/services/ubuntu/revision", revision, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/services/ubuntu", `
{
	"Container" : {
		"Config": {
			"Env": [
				"NODE_ENV=staging"
			]
		}
	}
}`, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/authoritative_names", `["ubuntu"]`, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/haproxy_config", "foobar", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/haproxy_files", "", &etcd.SetOptions{Dir: true})
	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/haproxy_files/hello.txt", "hello", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	_, err = etcdClient.Set(context.Background(), "/machineconfigurations/tags/testtag/certs/test.pem", "----TEST-----", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	tags := []string{"testtag"}
	var containrunner Containrunner
	configuration, err := containrunner.GetMachineConfigurationByTags(etcdClient, tags, "")

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

	_, _ = etcdClient.Delete(context.Background(), "/machineconfigurations/tags/testtag/", &etcd.DeleteOptions{Recursive: true})

}
func TestConfigResultEtcdPublisherServiceOk(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcdClient}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)

	res, err := etcdClient.Get(context.Background(), "/test/services/testService/endpoints/10.1.2.3:1234", nil)
	assert.Nil(t, err)
	assert.Equal(t, res.Node.Value, "null")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	assert.Equal(t, res.Node.TTL, int64(5))

	_, _ = etcdClient.Delete(context.Background(), "/test/services/testService/endpoints/10.1.2.3:1234", &etcd.DeleteOptions{Recursive: true})
}

func TestConfigResultEtcdPublisherServiceWithEndpointInfo(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcdClient}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, &EndpointInfo{
		Revision: "asdf",
	})

	res, err := etcdClient.Get(context.Background(), "/test/services/testService/endpoints/10.1.2.3:1234", nil)
	assert.Nil(t, err)

	endpointInfo := EndpointInfo{}
	err = json.Unmarshal([]byte(res.Node.Value), &endpointInfo)
	assert.Nil(t, err)
	assert.Equal(t, endpointInfo.Revision, "asdf")

	// Note that TTL counts down to zero, so if the machine is under heavy load then the TTL might not be anymore 5
	assert.Equal(t, res.Node.TTL, int64(5))

	_, _ = etcdClient.Delete(context.Background(), "/test/services/testService/endpoints/10.1.2.3:1234", &etcd.DeleteOptions{Recursive: true})
}

func TestConfigResultEtcdPublisherServiceNotOk(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcdClient}
	crep.PublishServiceState("testService", "10.1.2.3:1234", false, nil)

	_, err := etcdClient.Get(context.Background(), "/test/services/testService/endpoints/10.1.2.3:1234", nil)
	assert.NotNil(t, err)
}

func TestConfigResultEtcdPublisherWithPreviousExistingValue(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	crep := ConfigResultEtcdPublisher{5, "/test", nil, etcdClient}
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)
	crep.PublishServiceState("testService", "10.1.2.3:1234", true, nil)
	_, _ = etcdClient.Delete(context.Background(), "/test/services/testService/endpoints/0.1.2.3:1234", &etcd.DeleteOptions{Recursive: true})

}

func TestGetEndpointsForService(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	_, err := etcdClient.Set(context.Background(), "/test/services/testService2/endpoints/10.1.2.3:1234", "{\"Revision\":\"foobar\"}", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	containrunner.EtcdEndpoints = TestingEtcdEndpoints
	endpoints, err := containrunner.GetEndpointsForService("testService2")
	assert.Nil(t, err)

	assert.Equal(t, endpoints["10.1.2.3:1234"].Revision, "foobar")

}

func TestGetAllEndpoints(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	_, err := etcdClient.Set(context.Background(), "/test/services/testService2/endpoints/10.1.2.3:1234", "{\"Revision\":\"foo\"}", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)
	_, err = etcdClient.Set(context.Background(), "/test/services/testService2/endpoints/10.1.2.4:1234", "{\"Revision\":\"bar\"}", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)
	_, err = etcdClient.Set(context.Background(), "/test/services/testService1/endpoints/10.1.2.4:1000", "{\"Revision\":\"kissa\"}", &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	serviceEndpoints, err := GetAllServiceEndpoints(TestingEtcdEndpoints, "/test")
	assert.Nil(t, err)

	assert.Equal(t, serviceEndpoints["testService1"]["10.1.2.4:1000"].Revision, "kissa")
	assert.Equal(t, serviceEndpoints["testService2"]["10.1.2.3:1234"].Revision, "foo")
	assert.Equal(t, serviceEndpoints["testService2"]["10.1.2.4:1234"].Revision, "bar")

}

func TestGetAllServices(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	_, err := etcdClient.Set(context.Background(), "/test/services/testService2/config", `{
"Name" : "testService2",
"EndpointPort" : 1025
}`, &etcd.SetOptions{TTL: 10})
	assert.Nil(t, err)

	var containrunner Containrunner
	var services map[string]ServiceConfiguration
	containrunner.EtcdBasePath = "/test"
	services, err = containrunner.GetAllServices(etcdClient)
	assert.Nil(t, err)

	assert.Equal(t, services["testService2"].Name, "testService2")
	assert.Equal(t, services["testService2"].EndpointPort, 1025)

}

func TestTagServiceToTag(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	_, _ = etcdClient.Delete(context.Background(), "/test/machineconfigurations/tags/testtag/services/myservice", &etcd.DeleteOptions{Recursive: true})

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	err := containrunner.TagServiceToTag("myservice", "testtag", etcdClient)
	assert.Nil(t, err)

	res, err := etcdClient.Get(context.Background(), "/test/machineconfigurations/tags/testtag/services/myservice", nil)
	assert.Nil(t, err)
	assert.Equal(t, res.Node.Value, "{}")

	_, _ = etcdClient.Delete(context.Background(), "/test/machineconfigurations/tags/testtag/services/myservice", &etcd.DeleteOptions{Recursive: true})

}

func TestGetServiceRevision(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	etcdClient.Delete(context.Background(), "/test/services/myservice/revision", &etcd.DeleteOptions{Recursive: true})
	etcdClient.Set(context.Background(), "/test/services/myservice/revision", `
{
	"Revision" : "asdfasdf"
}`, &etcd.SetOptions{TTL: 10})

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	serviceRevision, err := containrunner.GetServiceRevision("myservice", etcdClient)
	assert.Nil(t, err)
	assert.Equal(t, serviceRevision.Revision, "asdfasdf")

	etcdClient.Delete(context.Background(), "/test/services/myservice/revision", &etcd.DeleteOptions{Recursive: true})

}

func TestGetServiceByName(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	etcdClient.Delete(context.Background(), "/test/services/myservice/revision", &etcd.DeleteOptions{Recursive: true})
	etcdClient.Set(context.Background(), "/test/services/myservice/revision", `
{
	"Revision" : "asdfasdf"
}`, &etcd.SetOptions{TTL: 10})

	etcdClient.Delete(context.Background(), "/test/services/myservice/machines", &etcd.DeleteOptions{Recursive: true})
	etcdClient.Set(context.Background(), "/test/services/myservice/machines/10.0.0.1", `
{
	"Revision" : "newrevision"
}`, &etcd.SetOptions{TTL: 10})

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"
	serviceConfiguration, err := containrunner.GetServiceByName("myservice", etcdClient, "10.0.0.1")
	assert.Nil(t, err)
	assert.Equal(t, serviceConfiguration.Revision.Revision, "newrevision")

	etcdClient.Delete(context.Background(), "/test/services/myservice/revision", &etcd.DeleteOptions{Recursive: true})
	etcdClient.Delete(context.Background(), "/test/services/myservice/machines/10.0.0.1", &etcd.DeleteOptions{Recursive: true})
	etcdClient.Delete(context.Background(), "/test/services/myservice/machines", &etcd.DeleteOptions{Recursive: true})

}

func TestSetServiceRevision(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	etcdClient.Delete(context.Background(), "/test/services/myservice/revision", &etcd.DeleteOptions{Recursive: true})
	etcdClient.Set(context.Background(), "/test/services/myservice/machines/10.0.0.1", `
{
	"Revision" : "newrevision"
}`, &etcd.SetOptions{TTL: 10})

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"

	serviceRevision := ServiceRevision{
		Revision: "asdf",
	}
	var serviceRevision2 ServiceRevision

	err := containrunner.SetServiceRevision("myservice", serviceRevision, etcdClient)
	assert.Nil(t, err)

	res, err := etcdClient.Get(context.Background(), "/test/services/myservice/revision", nil)
	assert.Nil(t, err)

	err = json.Unmarshal([]byte(res.Node.Value), &serviceRevision2)
	assert.Nil(t, err)

	assert.Equal(t, serviceRevision2.Revision, "asdf")

	res, err = etcdClient.Get(context.Background(), "/test/services/myservice/machines/10.0.0.1", nil)
	assert.NotNil(t, err)

	etcdClient.Delete(context.Background(), "/test/services/myservice/revision", &etcd.DeleteOptions{Recursive: true})
}

func TestSetServiceRevisionForMachine(t *testing.T) {
	etcdClient := GetTestingEtcdClient()
	etcdClient.Delete(context.Background(), "/test/", &etcd.DeleteOptions{Recursive: true})

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test"

	serviceRevision := ServiceRevision{
		Revision: "asdf",
	}
	var serviceRevision2 ServiceRevision

	err := containrunner.SetServiceRevisionForMachine("myservice", serviceRevision, "10.0.0.2", etcdClient)
	assert.Nil(t, err)

	res, err := etcdClient.Get(context.Background(), "/test/services/myservice/machines/10.0.0.2", nil)
	assert.Nil(t, err)

	err = json.Unmarshal([]byte(res.Node.Value), &serviceRevision2)
	assert.Nil(t, err)

	assert.Equal(t, serviceRevision2.Revision, "asdf")

	etcdClient.Delete(context.Background(), "/test/services/myservice/machines", &etcd.DeleteOptions{Recursive: true})
}
