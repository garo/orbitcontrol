package containrunner

import (
	"github.com/coreos/go-etcd/etcd"
	. "gopkg.in/check.v1"
)

type VerifySuite struct {
	etcd *etcd.Client
}

var _ = Suite(&VerifySuite{})

func (s *VerifySuite) SetUpTest(c *C) {
	s.etcd = etcd.NewClient(TestingEtcdEndpoints)

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryTesttagMissing(c *C) {
	s.etcd.Delete("/test2/", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.Services = nil
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/machineconfigurations/tags/testtag")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceUbuntuMissing(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.MachineConfigurations = nil
	err = containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/services/ubuntu")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceubuntuConfigMissing(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/services/ubuntu/", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.MachineConfigurations = nil
	err = containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/services/ubuntu/config")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceubuntuConfigInvalidData(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/services/ubuntu/", 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/services/ubuntu/config", "", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.MachineConfigurations = nil
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "invalid content: /test2/services/ubuntu/config")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceubuntuConfiOk(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/services/ubuntu/", 10)

	var ubuntu = `{"Name":"ubuntu","EndpointPort":3500,"Checks":[{"Type":"http","Url":"http://127.0.0.1:3500/check","HttpHost":"","Username":"","Password":"","HostPort":"","DummyResult":false,"ExpectHttpStatus":"","ExpectString":"","ConnectTimeout":0,"ResponseTimeout":0}],"Container":{"HostConfig":{"Binds":["/tmp:/data"],"ContainerIDFile":"","LxcConf":null,"Privileged":false,"PortBindings":null,"Links":null,"PublishAllPorts":false,"Dns":null,"DnsSearch":null,"VolumesFrom":null,"NetworkMode":"host"},"Config":{"Hostname":"","Domainname":"","User":"","Memory":0,"MemorySwap":0,"CpuShares":0,"AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"PortSpecs":null,"ExposedPorts":null,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["NODE_ENV=vagrant"],"Cmd":null,"Dns":null,"Image":"ubuntu","Volumes":null,"VolumesFrom":"","WorkingDir":"","Entrypoint":null,"NetworkDisabled":false}},"Revision":null,"SourceControl":{"Origin":"github.com/Applifier/ubuntu","OAuthToken":"","CIUrl":""},"Attributes":{}}`
	s.etcd.Set(containrunner.EtcdBasePath+"/services/ubuntu/config", ubuntu, 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.MachineConfigurations = nil
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err, Equals, nil)

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagServiceDirMissing(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/machineconfigurations/tags/testtag/services")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagServiceDirServicesMissing(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/machineconfigurations/tags/testtag/services/ubuntu")
}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagServiceDirServicesInvalidContent(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services", 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/ubuntu", "asdf", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "invalid json: /test2/machineconfigurations/tags/testtag/services/ubuntu")
}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagServiceDirServicesInvalidContent2(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services", 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/ubuntu", "{}", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "invalid content: /test2/machineconfigurations/tags/testtag/services/ubuntu")
}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagServiceDirServicesInvalidContent3(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services", 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/ubuntu", `
{
	"Container" : {
		"Config": {
		"Env": [

				"NODE_ENV=this shall be invalid"
		],
			"Image":"latest",

			"Hostname": "ubuntu-test"
		}
	}
}

		`, 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "invalid content: /test2/machineconfigurations/tags/testtag/services/ubuntu")
}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagServiceDirServicesOk(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services", 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/ubuntu", `
{
	"Container" : {
		"Config": {
		"Env": [

				"NODE_ENV=staging"
		],
			"Image":"ubuntu",

			"Hostname": "ubuntu-test"
		}
	}
}

		`, 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.Services = nil
	tmp := localoc.MachineConfigurations["testtag"]
	tmp.HAProxyConfiguration = nil
	localoc.MachineConfigurations["testtag"] = tmp
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err, Equals, nil)
}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagHaproxyTemplateIsMissing(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services", 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/ubuntu", `
{
	"Container" : {
		"Config": {
		"Env": [

				"NODE_ENV=staging"
		],
			"Image":"ubuntu",

			"Hostname": "ubuntu-test"
		}
	}
}

		`, 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.Services = nil
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/machineconfigurations/tags/testtag/haproxy_config")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceTagHaproxyTemplateIsInvalid(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services", 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/ubuntu", `
{
	"Container" : {
		"Config": {
		"Env": [

				"NODE_ENV=staging"
		],
			"Image":"ubuntu",

			"Hostname": "ubuntu-test"
		}
	}
}

		`, 10)
	s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/haproxy_config", "wrong", 10)

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	localoc.Services = nil
	err := containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err.Error(), Equals, "invalid content: /test2/machineconfigurations/tags/testtag/haproxy_config")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryNoProblems(c *C) {
	s.etcd.Delete("/test3/", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test3"

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		s.etcd.DeleteDir(containrunner.EtcdBasePath + "/machineconfigurations/tags/testtag/")
	}

	var ubuntu = `{"Name":"ubuntu","EndpointPort":3500,"Checks":[{"Type":"http","Url":"http://127.0.0.1:3500/check","HttpHost":"","Username":"","Password":"","HostPort":"","DummyResult":false,"ExpectHttpStatus":"","ExpectString":"","ConnectTimeout":0,"ResponseTimeout":0}],"Container":{"HostConfig":{"Binds":["/tmp:/data"],"ContainerIDFile":"","LxcConf":null,"Privileged":false,"PortBindings":null,"Links":null,"PublishAllPorts":false,"Dns":null,"DnsSearch":null,"VolumesFrom":null,"NetworkMode":"host"},"Config":{"Hostname":"","Domainname":"","User":"","Memory":0,"MemorySwap":0,"CpuShares":0,"AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"PortSpecs":null,"ExposedPorts":null,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["NODE_ENV=vagrant"],"Cmd":null,"Dns":null,"Image":"ubuntu","Volumes":null,"VolumesFrom":"","WorkingDir":"","Entrypoint":null,"NetworkDisabled":false}},"Revision":null,"SourceControl":{"Origin":"github.com/Applifier/ubuntu","OAuthToken":"","CIUrl":""},"Attributes":{}}`
	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/services/ubuntu/config", ubuntu, 10)
	if err != nil {
		panic(err)
	}

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/services/ubuntu/revision", revision, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/ubuntu", `{
	"Container" : {
		"Config": {
			"Env": [
				"NODE_ENV=staging"
			],
			"Image":"ubuntu",
			"Hostname": "ubuntu-test"
		}
	}
}
`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/authoritative_names", `["ubuntu"]`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/haproxy_config", `global
stats socket /var/run/haproxy/admin.sock level admin user haproxy group haproxy
	log 127.0.0.1	local2 info
	maxconn 16000
 	ulimit-n 40000
	user haproxy
	group haproxy
	daemon
	quiet
	pidfile /var/run/haproxy.pid

defaults
	log	global
	mode http
	option httplog
	option dontlognull
	retries 3
	option redispatch
	maxconn	8000
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000

`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/haproxy_files", 10)
	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/haproxy_files/500.http", `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/haproxy_files/hello.txt", "hello", 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/certs/test.pem", "----TEST-----", 10)
	if err != nil {
		panic(err)
	}

	localoc, _ := containrunner.LoadOrbitConfigurationFromFiles("../testdata")
	err = containrunner.VerifyAgainstConfiguration(localoc)
	c.Assert(err, Equals, nil)

}
