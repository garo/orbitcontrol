package containrunner

import . "gopkg.in/check.v1"

//import "fmt"
//import "strings"

type HAProxySuite struct {
}

var _ = Suite(&HAProxySuite{})

func (s *HAProxySuite) SetUpTest(c *C) {
}

func (s *HAProxySuite) TestGetNewConfig(c *C) {
	var hac HAProxyConfiguration
	hac.GlobalSection = "foo\nbar"

	str, err := hac.GetNewConfig()
	c.Assert(err, IsNil)

	c.Assert(str, Equals, "foo\nbar\n")
}

func (s *HAProxySuite) TestBuildAndVerifyNewConfigWithErrors(c *C) {
	var hac HAProxyConfiguration
	hac.GlobalSection =
		`
global
s
`
	hac.HAProxyBinary = "/usr/sbin/haproxy"

	err := hac.BuildAndVerifyNewConfig()
	c.Assert(err, Not(IsNil))
}

func (s *HAProxySuite) TestBuildAndVerifyNewConfig(c *C) {
	var hac HAProxyConfiguration
	hac.GlobalSection =
		`
listen test 127.0.0.1:80
	mode http
	backend 127.0.0.1:81
`
	hac.HAProxyBinary = "/usr/sbin/haproxy"
	hac.HAProxyConfigPath = "/tmp"
	hac.HAProxyConfigName = "haproxy_config_test.cfg"

	err := hac.BuildAndVerifyNewConfig()
	c.Assert(err, IsNil)
}

func (s *HAProxySuite) TestGetServicesSectionOneListen(c *C) {
	var hac HAProxyConfiguration
	hac.Services = make(map[string]HAProxyEndpoint)

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
	hac.Services["comet"] = service

	str, err := hac.GetServicesSection()

	c.Assert(err, IsNil)
	c.Assert(str, Equals,
		`listen comet 127.0.0.1:89
	mode http
	balance leastconn
	server 10.0.0.1:1234 check inter 2000 rise 2 fall 2 maxconn 40

`)

}

func (s *HAProxySuite) TestGetServicesSectionOneBackend(c *C) {
	var hac HAProxyConfiguration
	hac.Services = make(map[string]HAProxyEndpoint)

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
	hac.Services["comet"] = service

	str, err := hac.GetServicesSection()

	c.Assert(err, IsNil)
	c.Assert(str, Equals,
		`backend comet
	mode http
	balance leastconn
	server 10.0.0.1:1234 check inter 2000 rise 2 fall 2 maxconn 40

`)

}
