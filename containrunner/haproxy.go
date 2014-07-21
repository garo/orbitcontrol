package containrunner

import "errors"
import "fmt"
import "strings"
import "os/exec"
import "os"
import "io/ioutil"
import "time"

// Static HAProxy settings
type HAProxySettings struct {
	GlobalSection        string
	HAProxyBinary        string
	HAProxyConfigPath    string
	HAProxyConfigName    string
	HAProxyReloadCommand string
}

// Dynamic HAProxy settings receivered from configbridge
type HAProxyConfiguration struct {
	Endpoints map[string]*HAProxyEndpoint
}

type HAProxyEndpoint struct {
	Name           string
	BackendServers map[string]string
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

func (hac *HAProxySettings) ConvergeHAProxy(configuration *HAProxyConfiguration) error {
	log.Info(LogString("ConvergeHAProxy running"))
	fmt.Printf("HAProxyConfiguration: %+v\n", configuration)

	err := hac.BuildAndVerifyNewConfig(configuration)
	if err != nil {
		log.Error(LogString("Error building new HAProxy configuration"))

		return err
	}

	err = hac.ReloadHAProxy()

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
	defer os.Remove(new_config.Name())
	config, err := hac.GetNewConfig(configuration)
	if err != nil {
		return err
	}

	fmt.Println(config)

	new_config.WriteString(config)
	new_config.Close()

	cmd := exec.Command(hac.HAProxyBinary, "-c", "-f", new_config.Name())
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
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
	str := hac.GlobalSection + "\n"
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
