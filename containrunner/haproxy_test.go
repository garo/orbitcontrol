package containrunner

import (
	_ "fmt"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"os"
)

type HAProxySuite struct {
}

var _ = Suite(&HAProxySuite{})

func (s *HAProxySuite) SetUpTest(c *C) {
}

func (s *HAProxySuite) TestConvergeHAProxy(c *C) {
	var settings HAProxySettings
	settings.GlobalSection = `
defaults
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000
`
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigName = "haproxy-test-config.cfg"
	settings.HAProxyConfigPath = "/tmp"

	configuration := NewHAProxyConfiguration()
	endpoint := NewHAProxyEndpoint()
	endpoint.Name = "test"
	endpoint.BackendServers["10.0.0.1:80"] = ""
	endpoint.Config.ListenAddress = "127.0.0.1:80"
	endpoint.Config.Listen = `
mode http
`
	configuration.Endpoints["test"] = endpoint

	_, err := os.Stat("/tmp/haproxy-test-config.cfg")
	if err == nil {
		os.Remove("/tmp/haproxy-test-config.cfg")
	}

	err = settings.ConvergeHAProxy(configuration)
	c.Assert(err, IsNil)

	var bytes []byte
	bytes, err = ioutil.ReadFile("/tmp/haproxy-test-config.cfg")
	c.Assert(err, IsNil)

	str := string(bytes)
	c.Assert(str, Equals, `
defaults
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000

listen test 127.0.0.1:80
	mode http
	server test-10.0.0.1:80 10.0.0.1:80

`)

	os.Remove("/tmp/haproxy-test-config.cfg")

}

func (s *HAProxySuite) TestReloadHAProxyOk(c *C) {
	var settings HAProxySettings
	settings.HAProxyReloadCommand = "/bin/true"

	err := settings.ReloadHAProxy()
	c.Assert(err, IsNil)
}

func (s *HAProxySuite) TestReloadHAProxyError(c *C) {
	var settings HAProxySettings
	settings.HAProxyReloadCommand = "/bin/false"

	err := settings.ReloadHAProxy()
	c.Assert(err, Not(IsNil))
}

func (s *HAProxySuite) TestGetNewConfig(c *C) {
	var settings HAProxySettings
	settings.GlobalSection = "foo\nbar"

	str, err := settings.GetNewConfig(nil)
	c.Assert(err, IsNil)

	c.Assert(str, Equals, "foo\nbar\n")
}

func (s *HAProxySuite) TestBuildAndVerifyNewConfigWithErrors(c *C) {
	var settings HAProxySettings
	settings.GlobalSection =
		`
global
s
`
	settings.HAProxyBinary = "/usr/sbin/haproxy"

	err := settings.BuildAndVerifyNewConfig(nil)
	c.Assert(err, Not(IsNil))
}

func (s *HAProxySuite) TestBuildAndVerifyNewConfig(c *C) {
	var settings HAProxySettings
	settings.GlobalSection =
		`
listen test 127.0.0.1:80
	mode http
	backend 127.0.0.1:81
`
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigPath = "/tmp"
	settings.HAProxyConfigName = "haproxy_config_test.cfg"

	err := settings.BuildAndVerifyNewConfig(nil)
	c.Assert(err, IsNil)
}

func (s *HAProxySuite) TestGetServicesSectionOneListen(c *C) {
	var settings HAProxySettings
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.Config = HAProxyEndpointConfiguration{}

	service.Config.PerServer = "check inter 2000 rise 2 fall 2 maxconn 40"
	service.Config.ListenAddress = "127.0.0.1:89"
	service.Config.Listen =
		`mode http
balance leastconn
`
	service.BackendServers = make(map[string]string)
	service.BackendServers["10.0.0.1:1234"] = ""
	configuration.Endpoints["comet"] = &service

	str, err := settings.GetServicesSection(configuration)

	c.Assert(err, IsNil)
	c.Assert(str, Equals,
		`listen comet 127.0.0.1:89
	mode http
	balance leastconn
	server comet-10.0.0.1:1234 10.0.0.1:1234 check inter 2000 rise 2 fall 2 maxconn 40

`)

}

func (s *HAProxySuite) TestGetServicesSectionOneBackend(c *C) {
	var settings HAProxySettings
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.Config = HAProxyEndpointConfiguration{}

	service.Config.PerServer = "check inter 2000 rise 2 fall 2 maxconn 40"
	service.Config.Backend =
		`mode http
balance leastconn
`
	service.BackendServers = make(map[string]string)
	service.BackendServers["10.0.0.1:1234"] = ""
	configuration.Endpoints["comet"] = &service

	str, err := settings.GetServicesSection(configuration)

	c.Assert(err, IsNil)
	c.Assert(str, Equals,
		`backend comet
	mode http
	balance leastconn
	server comet-10.0.0.1:1234 10.0.0.1:1234 check inter 2000 rise 2 fall 2 maxconn 40

`)

}
