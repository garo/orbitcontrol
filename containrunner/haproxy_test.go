package containrunner

import (
	"bufio"
	"fmt"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"net"
	"os"
)

type HAProxySuite struct {
}

var _ = Suite(&HAProxySuite{})

func (s *HAProxySuite) SetUpTest(c *C) {
}

func (s *HAProxySuite) TestConvergeHAProxy(c *C) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigName = "haproxy-test-config.cfg"
	settings.HAProxyConfigPath = "/tmp"

	configuration := NewHAProxyConfiguration()
	configuration.GlobalSection = `
defaults
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000
`

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

	err = settings.ConvergeHAProxy(configuration, nil)
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
	configuration := NewHAProxyConfiguration()
	configuration.GlobalSection = "foo\nbar"

	str, err := settings.GetNewConfig(configuration)
	c.Assert(err, IsNil)

	c.Assert(str, Equals, "foo\nbar\n")
}

func (s *HAProxySuite) TestBuildAndVerifyNewConfigWithErrors(c *C) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	configuration := NewHAProxyConfiguration()
	configuration.GlobalSection = "foo\nbar"

	err := settings.BuildAndVerifyNewConfig(configuration)
	c.Assert(err, Not(IsNil))
}

func (s *HAProxySuite) TestBuildAndVerifyNewConfig(c *C) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigPath = "/tmp"
	settings.HAProxyConfigName = "haproxy_config_test.cfg"

	configuration := NewHAProxyConfiguration()
	configuration.GlobalSection = `
listen test 127.0.0.1:80
	mode http
	backend 127.0.0.1:81
`

	err := settings.BuildAndVerifyNewConfig(configuration)
	c.Assert(err, IsNil)
}

func (s *HAProxySuite) TestGetServicesSectionOneListen(c *C) {
	var settings HAProxySettings
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
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

func (s *HAProxySuite) TestUpdateBackendsUpdateRequiredWithNewBackendSection(c *C) {
	fmt.Fprintf(os.Stderr, "TestUpdateBackendsUpdateRequiredWithNewBackendSection***\n")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.BackendServers = make(map[string]string)
	service.BackendServers["172.16.2.159:3500"] = ""
	configuration.Endpoints["comet"] = &service

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	if err != nil {
		panic(err)
	}
	defer os.Remove("/tmp/sock_srv")

	defer ln.Close()

	go func(ln net.Listener) {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			reader := bufio.NewReader(c)
			msg, err := reader.ReadBytes('\n')

			if string(msg) == "show stat\n" {
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, true)

}

func (s *HAProxySuite) TestUpdateBackendsUpdateRequiredWithNewEndpointInBackend(c *C) {
	fmt.Println("TestUpdateBackendsUpdateRequiredWithNewEndpointInBackend")

	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.BackendServers = make(map[string]string)
	service.BackendServers["172.16.2.159:3500"] = ""
	configuration.Endpoints["comet"] = &service

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	if err != nil {
		panic(err)
	}
	defer os.Remove("/tmp/sock_srv")

	defer ln.Close()

	go func(ln net.Listener) {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			reader := bufio.NewReader(c)
			msg, err := reader.ReadBytes('\n')

			if string(msg) == "show stat\n" {
				c.Write([]byte("comet,FRONTEND,,,0,0,8000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,\n"))
				c.Write([]byte("comet,comet-172.16.2.230:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, true)

}

func (s *HAProxySuite) TestUpdateBackendsNoUpdateRequiredEverythingMatches(c *C) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredEverythingMatches")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.BackendServers = make(map[string]string)
	service.BackendServers["172.16.2.159:3500"] = ""
	configuration.Endpoints["comet"] = &service

	var commands []string

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	if err != nil {
		panic(err)
	}
	defer os.Remove("/tmp/sock_srv")

	defer ln.Close()

	go func(ln net.Listener) {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			reader := bufio.NewReader(c)
			msg, err := reader.ReadBytes('\n')
			commands = append(commands, string(msg))

			if string(msg) == "show stat\n" {
				c.Write([]byte("comet,FRONTEND,,,0,0,8000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,\n"))
				c.Write([]byte("comet,comet-172.16.2.159:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, false)

	c.Assert(commands[0], Equals, "show stat\n")
}

func (s *HAProxySuite) TestUpdateBackendsNoUpdateRequiredButServerMustBeDisabled(c *C) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredButServerMustBeDisabled")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.BackendServers = make(map[string]string)
	service.BackendServers["172.16.2.159:3500"] = ""
	configuration.Endpoints["comet"] = &service

	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	if err != nil {
		panic(err)
	}
	defer os.Remove("/tmp/sock_srv")

	defer ln.Close()

	go func(ln net.Listener) {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			reader := bufio.NewReader(c)
			msg, err := reader.ReadBytes('\n')
			commands <- string(msg)
			fmt.Printf("Got command: %s", string(msg))

			if string(msg) == "show stat\n" {
				c.Write([]byte("comet,FRONTEND,,,0,0,8000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,\n"))
				c.Write([]byte("comet,comet-172.16.2.159:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.2.160:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, false)

	c.Assert(<-commands, Equals, "show stat\n")
	c.Assert(<-commands, Equals, "disable server comet/comet-172.16.2.160:3500\n")
}

func (s *HAProxySuite) TestUpdateBackendsNoUpdateRequiredButServerMustBeEnabled(c *C) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredButServerMustBeDisabled")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.BackendServers = make(map[string]string)
	service.BackendServers["172.16.2.159:3500"] = ""
	service.BackendServers["172.16.2.160:3500"] = ""
	configuration.Endpoints["comet"] = &service

	// Channel to get the haproxy status socket commands from our haproxy fake server into the test
	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	if err != nil {
		panic(err)
	}
	defer os.Remove("/tmp/sock_srv")

	defer ln.Close()

	go func(ln net.Listener) {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			reader := bufio.NewReader(c)
			msg, err := reader.ReadBytes('\n')
			commands <- string(msg)
			fmt.Printf("Got command: %s", string(msg))

			if string(msg) == "show stat\n" {
				c.Write([]byte("comet,FRONTEND,,,0,0,8000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,\n"))
				c.Write([]byte("comet,comet-172.16.2.159:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.2.160:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,MAINT,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, false)

	c.Assert(<-commands, Equals, "show stat\n")
	c.Assert(<-commands, Equals, "enable server comet/comet-172.16.2.160:3500\n")
}

func (s *HAProxySuite) TestUpdateBackendsNoUpdateRequiredButDownServerMustBeDisabled(c *C) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredButDownServerMustBeDisabled")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	service := HAProxyEndpoint{}
	service.Name = "comet"
	service.BackendServers = make(map[string]string)
	service.BackendServers["172.16.2.159:3500"] = ""
	configuration.Endpoints["comet"] = &service

	// Channel to get the haproxy status socket commands from our haproxy fake server into the test
	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	if err != nil {
		panic(err)
	}
	defer os.Remove("/tmp/sock_srv")

	defer ln.Close()

	go func(ln net.Listener) {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			reader := bufio.NewReader(c)
			msg, err := reader.ReadBytes('\n')
			commands <- string(msg)
			fmt.Printf("Got command: %s", string(msg))

			if string(msg) == "show stat\n" {
				c.Write([]byte("comet,FRONTEND,,,0,0,8000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,\n"))
				c.Write([]byte("comet,comet-172.16.2.159:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.2.160:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, false)

	c.Assert(<-commands, Equals, "show stat\n")
	c.Assert(<-commands, Equals, "disable server comet/comet-172.16.2.160:3500\n")
}
