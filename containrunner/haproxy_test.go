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

type ConfigBridgeInterfaceMock struct {
	Stub func(service_name string) (map[string]*EndpointInfo, error)
}

func (c ConfigBridgeInterfaceMock) GetHAProxyEndpointsForService(service_name string) (map[string]*EndpointInfo, error) {
	return c.Stub(service_name)
}

func (s *HAProxySuite) TestConvergeHAProxy(c *C) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigName = "haproxy-test-config.cfg"
	settings.HAProxyConfigPath = "/tmp"

	moc := ConfigBridgeInterfaceMock{
		func(service_name string) (map[string]*EndpointInfo, error) {
			endpoints := make(map[string]*EndpointInfo)
			endpoints["10.0.0.1:80"] = &EndpointInfo{}
			return endpoints, nil
		},
	}

	configuration := NewHAProxyConfiguration()
	configuration.Template = `
defaults
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000

listen test 127.0.0.1:80
	mode http
{{range Endpoints "test"}}
	server {{.Nickname}} {{.HostPort}}
{{end}}
`

	_, err := os.Stat("/tmp/haproxy-test-config.cfg")
	if err == nil {
		os.Remove("/tmp/haproxy-test-config.cfg")
	}

	err = settings.ConvergeHAProxy(moc, configuration, nil)
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

	moc := ConfigBridgeInterfaceMock{
		func(service_name string) (map[string]*EndpointInfo, error) {
			return nil, nil
		},
	}

	configuration := NewHAProxyConfiguration()
	configuration.Template = "foo\nbar"

	str, err := settings.GetNewConfig(moc, configuration)
	c.Assert(err, IsNil)

	c.Assert(str, Equals, "foo\nbar")
}

func (s *HAProxySuite) TestBuildAndVerifyNewConfigWithErrors(c *C) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	configuration := NewHAProxyConfiguration()
	configuration.Template = "foo\nbar"

	moc := ConfigBridgeInterfaceMock{
		func(service_name string) (map[string]*EndpointInfo, error) {
			return nil, nil
		},
	}

	config, err := settings.BuildAndVerifyNewConfig(moc, configuration)
	c.Assert(err, Not(IsNil))
	c.Assert(config, Equals, "")

}

func (s *HAProxySuite) TestBuildAndVerifyNewConfig(c *C) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigPath = "/tmp"
	settings.HAProxyConfigName = "haproxy_config_test.cfg"

	moc := ConfigBridgeInterfaceMock{
		func(service_name string) (map[string]*EndpointInfo, error) {
			return nil, nil
		},
	}

	configuration := NewHAProxyConfiguration()
	configuration.Template = `
listen test 127.0.0.1:80
	mode http
	backend 127.0.0.1:81
`
	configuration.Files["500.http"] = `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`

	config, err := settings.BuildAndVerifyNewConfig(moc, configuration)
	c.Assert(err, IsNil)
	c.Assert(config, Equals, configuration.Template)

	err = settings.CommitNewConfig(config, false)
	c.Assert(err, IsNil)

	bytes, err := ioutil.ReadFile(settings.HAProxyConfigPath + "/500.http")
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`)

}

func (s *HAProxySuite) TestUpdateBackendsUpdateRequiredWithNewBackendSection(c *C) {
	fmt.Fprintf(os.Stderr, "TestUpdateBackendsUpdateRequiredWithNewBackendSection***\n")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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

	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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

	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	backends["172.16.2.161:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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
				c.Write([]byte("comet,comet-172.16.2.161:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.2.162:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, false)

	c.Assert(<-commands, Equals, "show stat\n")
	c.Assert(<-commands, Equals, "disable server comet/comet-172.16.2.162:3500\n")
}

func (s *HAProxySuite) TestUpdateBackendsUpdateRequired_because_less_than80_percent_servers_are_up(c *C) {
	fmt.Println("TestUpdateBackendsUpdateRequired_because_less_than80_percent_servers_are_up")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"

	configuration := NewHAProxyConfiguration()
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	backends["172.16.2.161:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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
				c.Write([]byte("comet,comet-172.16.2.161:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.2.162:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.2.163:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, true)

	c.Assert(<-commands, Equals, "show stat\n")
}
func (s *HAProxySuite) TestUpdateBackendsNoUpdateRequiredButServerMustBeEnabled(c *C) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredButServerMustBeDisabled")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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

	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	backends["172.16.2.161:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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
				c.Write([]byte("comet,comet-172.16.2.161:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.2.162:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, false)

	c.Assert(<-commands, Equals, "show stat\n")
	c.Assert(<-commands, Equals, "disable server comet/comet-172.16.2.162:3500\n")
}

func (s *HAProxySuite) TestUpdateBackendsNoCheck(c *C) {
	fmt.Println("****** TestUpdateBackendsNoCheck")
	var settings HAProxySettings
	settings.HAProxySocket = "/tmp/sock_srv"
	configuration := NewHAProxyConfiguration()

	backends := make(map[string]*EndpointInfo)
	backends["172.16.3.159:3500"] = &EndpointInfo{}
	backends["172.16.3.160:3500"] = &EndpointInfo{}
	backends["172.16.3.161:3500"] = &EndpointInfo{}
	configuration.ServiceBackends["comet"] = backends

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
				c.Write([]byte("comet,comet-172.16.3.159:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.3.160:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,comet-172.16.3.161:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,nocheck-comet-172.16.2.159:3500,0,0,0,0,,0,0,0,,0,,0,0,0,0,DOWN,1,1,0,1,1,4,4,,1,2,2,,0,,2,0,,0,L4TOUT,,2002,0,0,0,0,0,0,0,,,,0,0,,,,,-1,,,0,0,0,0,\n"))
				c.Write([]byte("comet,BACKEND,0,0,0,0,800,0,0,0,0,0,,0,0,0,0,DOWN,0,0,0,,1,4,4,,1,2,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,\n"))
			}
			c.Close()
		}
	}(ln)

	restart_required, err := settings.UpdateBackends(configuration)

	c.Assert(err, IsNil)
	c.Assert(restart_required, Equals, false)

	c.Assert(<-commands, Equals, "show stat\n")
	select {
	case <-commands:
		c.Assert(true, Equals, false)
	default:
		c.Assert(true, Equals, true)
	}
}
