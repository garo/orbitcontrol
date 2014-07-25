package containrunner

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

type ConfigBridgeInterface interface {
	GetHAProxyEndpointsForService(service_name string) (map[string]string, error)
}

// Static HAProxy settings
type HAProxySettings struct {
	HAProxyBinary        string
	HAProxyConfigPath    string
	HAProxyConfigName    string
	HAProxyReloadCommand string
	HAProxySocket        string
}

// Dynamic HAProxy settings receivered from configbridge
type HAProxyConfiguration struct {
	Template string
	// First string is service name, second string is backend host:port
	ServiceBackends map[string]map[string]string `json:"-"`
}

type BackendParameters struct {
	Nickname string
	HostPort string
}

type HAProxyEndpoint struct {
	Name           string
	BackendServers map[string]string `json:"-"`
}

// Log structures
type HAProxyConfigError struct {
	Config string
	Error  string
}

type HAProxyConfigChangeLog struct {
	OldConfig           string
	NewConfig           string
	OldConfigBackupFile string
}

func NewHAProxyConfiguration() *HAProxyConfiguration {
	configuration := new(HAProxyConfiguration)
	configuration.ServiceBackends = make(map[string]map[string]string)

	return configuration
}

func NewHAProxyEndpoint() *HAProxyEndpoint {
	endpoint := new(HAProxyEndpoint)
	endpoint.BackendServers = make(map[string]string)

	return endpoint
}

func (hac *HAProxySettings) ConvergeHAProxy(cbi ConfigBridgeInterface, configuration *HAProxyConfiguration, oldConfiguration *HAProxyConfiguration) error {
	log.Info(LogString("ConvergeHAProxy running"))
	if configuration == nil {
		fmt.Fprintf(os.Stderr, "Error, HAProxy config is still nil!\n")
		return nil
	}

	err := hac.BuildAndVerifyNewConfig(cbi, configuration)
	if err != nil {
		log.Error(LogString("Error building new HAProxy configuration"))

		return err
	}

	reload_required, err := hac.UpdateBackends(configuration)
	if err != nil {
		log.Error(LogString(fmt.Sprintf("Error updating haproxy via stats socket. Error: %+v", err)))
		return err
	}

	if oldConfiguration != nil && oldConfiguration.Template != configuration.Template {
		fmt.Fprintf(os.Stderr, "Reloading haproxy because Template has changed")
		reload_required = true
	}

	if reload_required {
		err = hac.ReloadHAProxy()
	}

	return err
}

func (hac *HAProxySettings) ReloadHAProxy() error {
	if hac.HAProxyReloadCommand != "" {
		parts := strings.Fields(hac.HAProxyReloadCommand)
		head := parts[0]
		parts = parts[1:len(parts)]

		cmd := exec.Command(head, parts...)
		err := cmd.Start()
		if err != nil {
			panic(err)
		}

		err = cmd.Wait()
		return err

	}
	return nil
}

func (hac *HAProxySettings) BuildAndVerifyNewConfig(cbi ConfigBridgeInterface, configuration *HAProxyConfiguration) error {

	new_config, err := ioutil.TempFile(os.TempDir(), "haproxy_new_config_")
	if new_config != nil {
		defer os.Remove(new_config.Name())
	} else {
		fmt.Fprintf(os.Stderr, "Error: new_config was nil when creating temp file. Err: %+v\n", err)
	}

	config, err := hac.GetNewConfig(cbi, configuration)
	if err != nil {
		return err
	}

	//fmt.Println(config)

	new_config.WriteString(config)
	new_config.Close()

	cmd := exec.Command(hac.HAProxyBinary, "-c", "-f", new_config.Name())
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error (cmd.StderrPipe) verifying haproxy config with binary %s. Error: %+v\n", hac.HAProxyBinary, err)
		return err
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error (cmd.Start) verifying haproxy config with binary %s. Error: %+v\n", hac.HAProxyBinary, err)
		return err
	}

	stderr, err := ioutil.ReadAll(stderrPipe)
	err = cmd.Wait()

	if err != nil {
		log.Error(LogEvent(HAProxyConfigError{config, string(stderr)}))
		return errors.New("Invalid HAProxy configuration")
	}

	l := HAProxyConfigChangeLog{}
	var contents []byte
	contents, err = ioutil.ReadFile(hac.HAProxyConfigPath + "/" + hac.HAProxyConfigName)
	if err == nil {
		l.OldConfig = string(contents)
	}

	l.NewConfig = config
	l.OldConfigBackupFile = hac.HAProxyConfigPath + "/" + hac.HAProxyConfigName + "-" + time.Now().Format(time.RFC3339)

	err = os.Link(hac.HAProxyConfigPath+"/"+hac.HAProxyConfigName, l.OldConfigBackupFile)
	if err != nil && !os.IsNotExist(err) {
		log.Error(LogString("Error linking config backup!" + err.Error()))
		return err
	} else if err != nil && os.IsNotExist(err) {
		l.OldConfigBackupFile = ""
	}

	log.Debug(LogEvent(l))

	err = ioutil.WriteFile(hac.HAProxyConfigPath+"/"+hac.HAProxyConfigName, []byte(config), 0664)
	if err != nil {
		log.Error(LogString("Could not write new haproxy config!" + err.Error()))
		return err
	}

	return nil
}

func (hac *HAProxySettings) GetNewConfig(cbi ConfigBridgeInterface, configuration *HAProxyConfiguration) (string, error) {

	funcMap := template.FuncMap{
		// The name "title" is what the function will be called in the template text.
		"Endpoints": func(service_name string) ([]BackendParameters, error) {
			backend_servers, ok := configuration.ServiceBackends[service_name]
			var err error
			if ok == false {

				backend_servers, err = cbi.GetHAProxyEndpointsForService(service_name)

				if err != nil {
					return nil, err
				}
				configuration.ServiceBackends[service_name] = backend_servers
			}

			var backends []BackendParameters
			for hostport, _ := range backend_servers {
				backends = append(backends, BackendParameters{
					Nickname: service_name + "-" + hostport,
					HostPort: hostport,
				})
			}

			return backends, nil
		},
	}

	tmpl, err := template.New("main").Funcs(funcMap).Parse(configuration.Template)
	if err != nil {
		log.Fatalf("parsing: %s", err)
		return "", err
	}

	output := new(bytes.Buffer)
	// Run the template to verify the output.
	err = tmpl.Execute(output, "the go programming language")
	if err != nil {
		log.Fatalf("execution: %s", err)
		return "", err
	}

	return output.String(), nil
}

func (hac *HAProxySettings) UpdateBackends(configuration *HAProxyConfiguration) (bool, error) {
	c, err := net.Dial("unix", hac.HAProxySocket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening HAProxy socket. Error: %+v\n", err)
		if c != nil {
			c.Close()
		}
		return true, nil
	}
	defer c.Close()

	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	_, err = c.Write([]byte("show stat\n"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error on show stat command. Error: %+v\n", err)
		return true, nil
	}

	var bytes []byte
	bytes, err = ioutil.ReadAll(c)
	lines := strings.Split(string(bytes), "\n")

	c.Close()

	// Build list of currently existing backends in the running haproxy
	current_backends := make(map[string]map[string]string)
	enabled_backends := make(map[string][]string)

	for _, line := range lines {
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.Split(line, ",")
		//fmt.Printf("Read line: %+v\n", line)
		if parts[1] == "FRONTEND" || parts[1] == "BACKEND" {
			continue
		}

		if _, ok := current_backends[parts[0]]; ok == false {
			current_backends[parts[0]] = make(map[string]string)
		}
		current_backends[parts[0]][parts[1]] = parts[17]
	}

	//fmt.Printf("current backends: %+v\n", current_backends)

	for service_name, backend_servers := range configuration.ServiceBackends {
		if _, ok := current_backends[service_name]; ok == false {
			fmt.Printf("Restart required: missing section %s\n", service_name)
			return true, nil
		}
		for backendServer := range backend_servers {
			if _, ok := current_backends[service_name][service_name+"-"+backendServer]; ok == false {
				fmt.Printf("Restart required: missing endpoint %s from section %s\n", service_name+"-"+backendServer, service_name)
				return true, nil
			}
			enabled_backends[service_name] = append(enabled_backends[service_name], service_name+"-"+backendServer)
		}
	}
	fmt.Printf("enabled backends: %+v\n", enabled_backends)

	for section_name, section_backends := range current_backends {
		for backend, backend_status := range section_backends {
			command := ""
			if contains(enabled_backends[section_name], backend) == true {
				if backend_status == "MAINT" {
					command = "enable server " + section_name + "/" + backend + "\n"
				}
			} else {
				if backend_status != "MAINT" {
					command = "disable server " + section_name + "/" + backend + "\n"
				}
			}

			if command == "" {
				continue
			}

			fmt.Printf("executing command: %s", command)

			err := func(command string, socket_name string) error {
				c, err := net.Dial("unix", socket_name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error opening HAProxy socket. Error: %+v\n", err)
					if c != nil {
						c.Close()
					}
					return err
				}
				defer c.Close()

				_, err = c.Write([]byte(command))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error on show stat command. Error: %+v\n", err)
					return err
				}

				return nil
			}(command, hac.HAProxySocket)

			if err != nil {
				return true, err
			}
		}
	}

	return false, nil
}
