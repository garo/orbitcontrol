package containrunner

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

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
	GlobalSection string
	Endpoints     map[string]*HAProxyEndpoint `json:"-"`
}

type HAProxyEndpoint struct {
	Name           string
	BackendServers map[string]string `json:"-"`
	Config         struct {
		PerServer     string
		ListenAddress string
		Listen        string
		Backend       string
	}
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
	configuration.Endpoints = make(map[string]*HAProxyEndpoint)

	return configuration
}

func NewHAProxyEndpoint() *HAProxyEndpoint {
	endpoint := new(HAProxyEndpoint)
	endpoint.BackendServers = make(map[string]string)

	return endpoint
}

func (hac *HAProxySettings) ConvergeHAProxy(configuration *HAProxyConfiguration, oldConfiguration *HAProxyConfiguration) error {
	log.Info(LogString("ConvergeHAProxy running"))
	if configuration == nil {
		fmt.Fprintf(os.Stderr, "Error, HAProxy config is still nil!\n")
		return nil
	}

	err := hac.BuildAndVerifyNewConfig(configuration)
	if err != nil {
		log.Error(LogString("Error building new HAProxy configuration"))

		return err
	}

	reload_required, err := hac.UpdateBackends(configuration)
	if err != nil {
		log.Error(LogString(fmt.Sprintf("Error updating haproxy via stats socket. Error: %+v", err)))
		return err
	}

	if oldConfiguration != nil && oldConfiguration.GlobalSection != configuration.GlobalSection {
		fmt.Fprintf(os.Stderr, "Reloading haproxy because GlobalSection has changed")
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

func (hac *HAProxySettings) BuildAndVerifyNewConfig(configuration *HAProxyConfiguration) error {

	new_config, err := ioutil.TempFile(os.TempDir(), "haproxy_new_config_")
	if new_config != nil {
		defer os.Remove(new_config.Name())
	} else {
		fmt.Fprintf(os.Stderr, "Error: new_config was nil when creating temp file. Err: %+v\n", err)
	}

	config, err := hac.GetNewConfig(configuration)
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

func (hac *HAProxySettings) GetNewConfig(configuration *HAProxyConfiguration) (string, error) {
	str := configuration.GlobalSection + "\n"
	section, err := hac.GetServicesSection(configuration)
	if err != nil {
		return "", err
	}
	str += section

	return str, nil
}

func (hac *HAProxySettings) GetServicesSection(configuration *HAProxyConfiguration) (string, error) {
	str := ""

	if configuration == nil || configuration.Endpoints == nil {
		return str, nil
	}

	for name, service := range configuration.Endpoints {
		if name != service.Name {
			fmt.Printf("Service: %+v\n", service)
			return "", errors.New("Service name mismatch: " + name + " != " + service.Name)
		}

		if service.Config.Listen != "" && service.Config.ListenAddress != "" {
			str += "listen " + service.Name + " " + service.Config.ListenAddress + "\n"
			for _, line := range strings.Split(service.Config.Listen, "\n") {
				if line != "" && line != "\n" {
					str += "\t" + line + "\n"
				}
			}
		} else if service.Config.Backend != "" && service.Config.ListenAddress == "" {
			str += "backend " + service.Name + "\n"
			for _, line := range strings.Split(service.Config.Backend, "\n") {
				if line != "" && line != "\n" {
					str += "\t" + line + "\n"
				}
			}
		} else {
			return "", errors.New("Service Listen/Backend/ListenAddress mismatch or missing.")
		}
		for backendServer := range service.BackendServers {
			str += "\tserver " + name + "-" + backendServer + " " + backendServer
			if service.Config.PerServer != "" {
				str += " " + service.Config.PerServer
			}
			str += "\n"
		}

		str += "\n"

	}

	return str, nil
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

	for name, service := range configuration.Endpoints {
		if _, ok := current_backends[name]; ok == false {
			fmt.Printf("Restart required: missing section %s\n", name)
			return true, nil
		}
		for backendServer := range service.BackendServers {
			if _, ok := current_backends[name][name+"-"+backendServer]; ok == false {
				fmt.Printf("Restart required: missing endpoint %s from section %s\n", name+"-"+backendServer, name)
				return true, nil
			}
			enabled_backends[name] = append(enabled_backends[name], name+"-"+backendServer)
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
