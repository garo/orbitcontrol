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
	err := containrunner.VerifyAgainstLocalDirectory("../testdata")
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/machineconfigurations/tags/testtag")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceCometMissing(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)

	err = containrunner.VerifyAgainstLocalDirectory("../testdata")
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/services/comet")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceCometConfigMissing(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/services/comet/", 10)

	err = containrunner.VerifyAgainstLocalDirectory("../testdata")
	c.Assert(err.Error(), Equals, "etcd path missing: /test2/services/comet/config")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceCometConfigInvalidData(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/services/comet/", 10)
	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/services/comet/config", "", 10)

	err = containrunner.VerifyAgainstLocalDirectory("../testdata")
	c.Assert(err.Error(), Equals, "invalid content: /test2/services/comet/config")

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryServiceCometConfiOk(c *C) {
	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test2"

	s.etcd.Delete(containrunner.EtcdBasePath, true)

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		s.etcd.DeleteDir(containrunner.EtcdBasePath + "/machineconfigurations/tags/testtag/")
	}

	s.etcd.CreateDir(containrunner.EtcdBasePath+"/services/comet/", 10)

	var comet = `{"Name":"comet","EndpointPort":3500,"Checks":[{"Type":"http","Url":"http://127.0.0.1:3500/check","HttpHost":"","Username":"","Password":"","HostPort":"","DummyResult":false,"ExpectHttpStatus":"","ExpectString":""}],"Container":{"HostConfig":{"Binds":["/tmp:/data"],"ContainerIDFile":"","LxcConf":null,"Privileged":false,"PortBindings":null,"Links":null,"PublishAllPorts":false,"Dns":null,"DnsSearch":null,"VolumesFrom":null,"NetworkMode":"host"},"Config":{"Hostname":"","Domainname":"","User":"","Memory":0,"MemorySwap":0,"CpuShares":0,"AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"PortSpecs":null,"ExposedPorts":null,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["NODE_ENV=vagrant"],"Cmd":null,"Dns":null,"Image":"registry.applifier.info:5000/comet:8fd079b54719d61b6feafbb8056b9ba09ade4760","Volumes":null,"VolumesFrom":"","WorkingDir":"","Entrypoint":null,"NetworkDisabled":false}},"Revision":null,"SourceControl":{"Origin":"github.com/Applifier/comet","OAuthToken":"","CIUrl":""}}`
	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/services/comet/config", comet, 10)

	err = containrunner.VerifyAgainstLocalDirectory("../testdata")
	c.Assert(err, Equals, nil)

}

func (s *ConfigBridgeSuite) TestVerifyAgainstLocalDirectoryNoProblems(c *C) {
	s.etcd.Delete("/test3/", true)

	var containrunner Containrunner
	containrunner.EtcdBasePath = "/test3"

	_, err := s.etcd.CreateDir(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/", 10)
	if err != nil {
		s.etcd.DeleteDir(containrunner.EtcdBasePath + "/machineconfigurations/tags/testtag/")
	}

	var comet = `{"Name":"comet","EndpointPort":3500,"Checks":[{"Type":"http","Url":"http://127.0.0.1:3500/check","HttpHost":"","Username":"","Password":"","HostPort":"","DummyResult":false,"ExpectHttpStatus":"","ExpectString":""}],"Container":{"HostConfig":{"Binds":["/tmp:/data"],"ContainerIDFile":"","LxcConf":null,"Privileged":false,"PortBindings":null,"Links":null,"PublishAllPorts":false,"Dns":null,"DnsSearch":null,"VolumesFrom":null,"NetworkMode":"host"},"Config":{"Hostname":"","Domainname":"","User":"","Memory":0,"MemorySwap":0,"CpuShares":0,"AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"PortSpecs":null,"ExposedPorts":null,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["NODE_ENV=vagrant"],"Cmd":null,"Dns":null,"Image":"registry.applifier.info:5000/comet:8fd079b54719d61b6feafbb8056b9ba09ade4760","Volumes":null,"VolumesFrom":"","WorkingDir":"","Entrypoint":null,"NetworkDisabled":false}},"Revision":null,"SourceControl":{"Origin":"github.com/Applifier/comet","OAuthToken":"","CIUrl":""}}`
	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/services/comet/config", comet, 10)
	if err != nil {
		panic(err)
	}

	revision := `
{
	"Revision" : "asdf"
}`

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/services/comet/revision", revision, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/services/comet", `{
	"Container" : {
		"Config": {
			"Env": [
				"NODE_ENV=staging"
			],
			"Image":"registry.applifier.info:5000/comet:latest",
			"Hostname": "comet-test"
		}
	}
}
`, 10)
	if err != nil {
		panic(err)
	}

	_, err = s.etcd.Set(containrunner.EtcdBasePath+"/machineconfigurations/tags/testtag/authoritative_names", `["registry.applifier.info:5000/comet"]`, 10)
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

	err = containrunner.VerifyAgainstLocalDirectory("../testdata")
	c.Assert(err, Equals, nil)

}
