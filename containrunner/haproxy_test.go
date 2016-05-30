package containrunner

import (
	"bufio"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"os"
	"testing"
)

type ConfigBridgeInterfaceMock struct {
	Stub func(service_name string) (map[string]*EndpointInfo, error)
}

func (c ConfigBridgeInterfaceMock) GetEndpointsForService(service_name string) (map[string]*EndpointInfo, error) {
	return c.Stub(service_name)
}

func TestConvergeHAProxy(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigName = "haproxy-test-config.cfg"
	settings.HAProxyConfigPath = "/tmp"

	haProxyConfiguration := NewHAProxyConfiguration()
	haProxyConfiguration.Template = `
defaults
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000

listen test 127.0.0.1:80
	mode http
{{range Endpoints "test"}}
	server {{.Nickname}} {{.HostPort}}{{end}}

`
	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.ServiceBackends = make(map[string]map[string]*EndpointInfo)

	runtimeConfiguration.ServiceBackends["test"] = make(map[string]*EndpointInfo)
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"] = &EndpointInfo{}
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.1:80"] = &EndpointInfo{}
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.2:80"] = &EndpointInfo{}

	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration = haProxyConfiguration

	_, err := os.Stat("/tmp/haproxy-test-config.cfg")
	if err == nil {
		os.Remove("/tmp/haproxy-test-config.cfg")
	}

	err = settings.ConvergeHAProxy(&runtimeConfiguration, nil, "")
	assert.Nil(t, err)

	var bytes []byte
	bytes, err = ioutil.ReadFile("/tmp/haproxy-test-config.cfg")
	assert.Nil(t, err)

	str := string(bytes)
	assert.Equal(t, str, `
defaults
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000

listen test 127.0.0.1:80
	mode http

	server test-10.0.0.1:80 10.0.0.1:80
	server test-10.0.0.2:80 10.0.0.2:80
	server test-10.0.0.3:80 10.0.0.3:80

`)

	os.Remove("/tmp/haproxy-test-config.cfg")

}

func TestGetNewConfig2(t *testing.T) {
	var settings HAProxySettings

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.ServiceBackends = make(map[string]map[string]*EndpointInfo)
	runtimeConfiguration.ServiceBackends["test"] = make(map[string]*EndpointInfo)
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"] = &EndpointInfo{}
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"].Revision = "rev"
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"].ServiceConfiguration.Name = "name"
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"].ServiceConfiguration.Attributes = make(map[string]string)
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"].ServiceConfiguration.Attributes["weight"] = "8"

	runtimeConfiguration.ServiceBackends["test"]["10.0.0.4:80"] = &EndpointInfo{}

	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration = NewHAProxyConfiguration()
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration.Template = `
defaults
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000

listen test 127.0.0.1:80
	mode http
{{range Endpoints "test"}}
	HostPort: {{.HostPort}}
	Name: {{.ServiceConfiguration.Name}}
	Revision: {{.Revision}}
    Weight: {{if .ServiceConfiguration.Attributes.weight}}{{.ServiceConfiguration.Attributes.weight}}{{else}}10{{end}}
{{end}}

`

	_, err := settings.GetNewConfig(&runtimeConfiguration, "")
	assert.Nil(t, err)

}

func TestLocalEndpoints(t *testing.T) {
	var settings HAProxySettings

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.ServiceBackends = make(map[string]map[string]*EndpointInfo)
	runtimeConfiguration.ServiceBackends["test"] = make(map[string]*EndpointInfo)
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"] = &EndpointInfo{}
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"].Revision = "rev"
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.3:80"].AvailabilityZone = "zone1"

	runtimeConfiguration.ServiceBackends["test"]["10.0.0.4:80"] = &EndpointInfo{}
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.4:80"].Revision = "rev"
	runtimeConfiguration.ServiceBackends["test"]["10.0.0.4:80"].AvailabilityZone = "zone1"

	runtimeConfiguration.ServiceBackends["test"]["10.0.0.5:80"] = &EndpointInfo{}

	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration = NewHAProxyConfiguration()
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration.Template = `{{range LocalEndpoints "test"}}{{.HostPort}}{{end}}`

	str, err := settings.GetNewConfig(&runtimeConfiguration, "zone1")
	assert.Nil(t, err)

	assert.Equal(t, str, "10.0.0.3:8010.0.0.4:80")
}

func TestReloadHAProxyOk(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxyReloadCommand = "/bin/true"

	err := settings.ReloadHAProxy()
	assert.Nil(t, err)
}

func TestReloadHAProxyError(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxyReloadCommand = "/bin/false"

	err := settings.ReloadHAProxy()
	assert.NotNil(t, err)
}

func TestGetNewConfig(t *testing.T) {
	var settings HAProxySettings

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration = NewHAProxyConfiguration()
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration.Template = "foo\nbar"

	str, err := settings.GetNewConfig(&runtimeConfiguration, "")
	assert.Nil(t, err)

	assert.Equal(t, str, "foo\nbar")
}

func TestBuildAndVerifyNewConfigWithErrors(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration = NewHAProxyConfiguration()
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration.Template = "foo\nbar"

	config, err := settings.BuildAndVerifyNewConfig(&runtimeConfiguration, "")
	assert.NotNil(t, err)
	assert.Equal(t, config, "")

}

func TestBuildAndVerifyNewConfig(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxyBinary = "/usr/sbin/haproxy"
	settings.HAProxyConfigPath = "/tmp"
	settings.HAProxyConfigName = "haproxy_config_test.cfg"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration = NewHAProxyConfiguration()
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration.Template = `
listen test 127.0.0.1:80
	mode http
	backend 127.0.0.1:81
`
	runtimeConfiguration.MachineConfiguration.HAProxyConfiguration.Files["500.http"] = `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`

	config, err := settings.BuildAndVerifyNewConfig(&runtimeConfiguration, "")
	assert.Nil(t, err)
	assert.Equal(t, runtimeConfiguration.MachineConfiguration.HAProxyConfiguration.Template, config)

	err = settings.CommitNewConfig(config, false)
	assert.Nil(t, err)

	bytes, err := ioutil.ReadFile(settings.HAProxyConfigPath + "/500.http")
	assert.Nil(t, err)
	assert.Equal(t, string(bytes), `HTTP/1.0 500 Service Unavailable
Cache-Control: no-cache
Connection: close
Content-Type: text/html

`)

}

func TestUpdateBackendsUpdateRequiredWithNewBackendSection(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)
	assert.Equal(t, restart_required, true)

}

func TestUpdateBackendsUpdateRequiredWithNewEndpointInBackend(t *testing.T) {

	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)
	assert.Equal(t, restart_required, true)

}

func TestUpdateBackendsNoUpdateRequiredEverythingMatches(t *testing.T) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredEverythingMatches")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	var commands []string

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)
	assert.Equal(t, false, restart_required)

	assert.Equal(t, commands[0], "show stat\n")
}

func TestUpdateBackendsNoUpdateRequiredButServerMustBeDisabled(t *testing.T) {
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	backends["172.16.2.161:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)
	assert.Equal(t, false, restart_required)

	assert.Equal(t, <-commands, "show stat\n")
	assert.Equal(t, <-commands, "disable server comet/comet-172.16.2.162:3500\n")
}

func TestUpdateBackendsUpdateRequired_because_less_than80_percent_servers_are_up(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	backends["172.16.2.161:3500"] = &EndpointInfo{}
	backends["172.16.2.162:3500"] = &EndpointInfo{}
	backends["172.16.2.163:3500"] = &EndpointInfo{}
	backends["172.16.2.164:3500"] = &EndpointInfo{}
	backends["172.16.2.165:3500"] = &EndpointInfo{}
	backends["172.16.2.166:3500"] = &EndpointInfo{}
	backends["172.16.2.167:3500"] = &EndpointInfo{}
	backends["172.16.2.168:3500"] = &EndpointInfo{}
	backends["172.16.2.169:3500"] = &EndpointInfo{}
	backends["172.16.2.170:3500"] = &EndpointInfo{}
	backends["172.16.2.171:3500"] = &EndpointInfo{}
	backends["172.16.2.172:3500"] = &EndpointInfo{}
	backends["172.16.2.173:3500"] = &EndpointInfo{}
	backends["172.16.2.174:3500"] = &EndpointInfo{}
	backends["172.16.2.175:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)
	assert.Equal(t, true, restart_required)
	assert.Equal(t, <-commands, "show stat\n")
}
func TestUpdateBackendsNoUpdateRequiredButServerMustBeEnabled(t *testing.T) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredButServerMustBeDisabled")
	var settings HAProxySettings
	//settings.HAProxySocket = "/var/run/haproxy/admin.sock"
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	// Channel to get the haproxy status socket commands from our haproxy fake server into the test
	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)

	assert.Equal(t, false, restart_required)
	assert.Equal(t, <-commands, "show stat\n")
	assert.Equal(t, <-commands, "enable server comet/comet-172.16.2.160:3500\n")
}

func TestUpdateBackendsNoUpdateRequiredButDownServerMustBeDisabled(t *testing.T) {
	fmt.Println("TestUpdateBackendsNoUpdateRequiredButDownServerMustBeDisabled")
	var settings HAProxySettings
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	backends["172.16.2.161:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	// Channel to get the haproxy status socket commands from our haproxy fake server into the test
	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)
	assert.Equal(t, false, restart_required)
	assert.Equal(t, <-commands, "show stat\n")
	assert.Equal(t, <-commands, "disable server comet/comet-172.16.2.162:3500\n")

}

func TestUpdateBackendsNoCheck(t *testing.T) {
	fmt.Println("****** TestUpdateBackendsNoCheck")
	var settings HAProxySettings
	settings.HAProxySocket = "/tmp/sock_srv"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.3.159:3500"] = &EndpointInfo{}
	backends["172.16.3.160:3500"] = &EndpointInfo{}
	backends["172.16.3.161:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	// Channel to get the haproxy status socket commands from our haproxy fake server into the test
	commands := make(chan string, 5)

	ln, err := net.Listen("unix", "/tmp/sock_srv")
	assert.Nil(t, err)
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

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)

	assert.Equal(t, false, restart_required)
	assert.Equal(t, <-commands, "show stat\n")

	select {
	case <-commands:
		t.Fail()
	default:

	}
}

func TestUpdateMultipleBackends(t *testing.T) {
	var settings HAProxySettings
	settings.HAProxySocket = "/tmp/sock_srv*.sock"

	runtimeConfiguration := RuntimeConfiguration{}
	runtimeConfiguration.LocallyRequiredServices = make(map[string]map[string]*EndpointInfo)
	backends := make(map[string]*EndpointInfo)
	backends["172.16.2.159:3500"] = &EndpointInfo{}
	backends["172.16.2.160:3500"] = &EndpointInfo{}
	backends["172.16.2.161:3500"] = &EndpointInfo{}
	runtimeConfiguration.LocallyRequiredServices["comet"] = backends

	// Channel to get the haproxy status socket commands from our haproxy fake server into the test
	commands := make(chan string, 5)

	ln1, err := net.Listen("unix", "/tmp/sock_srv1.sock")
	if err != nil {
		panic(err)
	}
	defer os.Remove("/tmp/sock_srv1.sock")
	defer ln1.Close()

	ln2, err := net.Listen("unix", "/tmp/sock_srv2.sock")
	assert.Nil(t, err)
	defer os.Remove("/tmp/sock_srv2.sock")
	defer ln2.Close()

	fnc := func(ln net.Listener) {
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
	}

	go fnc(ln1)
	go fnc(ln2)

	restart_required, err := settings.UpdateBackends(&runtimeConfiguration)

	assert.Nil(t, err)
	assert.Equal(t, false, restart_required)
	assert.Equal(t, <-commands, "show stat\n")
	assert.Equal(t, <-commands, "disable server comet/comet-172.16.2.162:3500\n")
	assert.Equal(t, <-commands, "disable server comet/comet-172.16.2.162:3500\n")

}
