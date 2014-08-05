package containrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

/*
	Data model referred
	/orbit/env <-- file contents tells the environment
	/orbit/services/<name>/config
	/orbit/services/<name>/revision	// contains revision string inside which overwrites the set revision in /config
	/orbit/services/<name>/endpoints/<endpoint host:port>
	/orbit/machineconfigurations/tags/<tag>/authoritative_names
	/orbit/machineconfigurations/tags/<tag>/services/<service_name>				// Tags service to a tag
	/orbit/machineconfigurations/tags/<tag>/haproxy_endpoints/<service_name>

	Example:
	/orbit/<env>/machineconfigurations/tags/frontend-a/services/comet
	/orbit/<env>/machineconfigurations/tags/loadbalancer-a/haproxy_endpoints/comet


*/

// Represents the configuration of an entire Orbit Control system
type OrbitConfiguration struct {
	MachineConfigurations map[string]MachineConfiguration
	Services              map[string]ServiceConfiguration
}

// Represents a single tag inside a /orbit/machineconfiguration/
type TagConfiguration struct {
	Services             map[string]ServiceConfiguration `json:"services"`
	HAProxyConfiguration *HAProxyConfiguration
	AuthoritativeNames   []string `json:"authoritative_names"`
}

// Represents all configurations for a single physical machine.
// Because all tags are unioned into one this can be represented
// with just a single unioned TagConfiguration but its kept as a separated
// type
type MachineConfiguration struct {
	TagConfiguration
}

type ServiceConfiguration struct {
	Name         string
	EndpointPort int
	Checks       []ServiceCheck
	Container    *ContainerConfiguration
	Revision     *ServiceRevision
}

type ServiceRevision struct {
	Revision string
}

type ConfigResultPublisher interface {
	PublishServiceState(serviceName string, endpoint string, ok bool)
}

type ConfigResultEtcdPublisher struct {
	ttl           uint64
	EtcdBasePath  string
	EtcdEndpoints []string
	etcd          *etcd.Client
}

// Log events
type ServiceStateChangeEvent struct {
	ServiceName string
	Endpoint    string
	IsUp        bool
}

func (c *ConfigResultEtcdPublisher) PublishServiceState(serviceName string, endpoint string, ok bool) {
	if c.etcd == nil {
		fmt.Fprintf(os.Stderr, "Creating new Etcd client so that we can report that service %s at %s is %+v\n", serviceName, endpoint, ok)
		c.etcd = GetEtcdClient(c.EtcdEndpoints)
		fmt.Fprintf(os.Stderr, "Client created\n")

	}

	key := c.EtcdBasePath + "/services/" + serviceName + "/endpoints/" + endpoint

	_, err := c.etcd.Get(key, false, false)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		fmt.Fprintf(os.Stderr, "Error getting key %s from etcd. Error: %+v\n", key, err)
		c.etcd = nil
		return
	}
	if ok == true && err != nil && strings.HasPrefix(err.Error(), "100:") {
		// Key did not exists so we need to add the key
		log.Info(LogEvent(ServiceStateChangeEvent{serviceName, endpoint, ok}))
	} else if ok == false {
		if err == nil {
			log.Info(LogEvent(ServiceStateChangeEvent{serviceName, endpoint, ok}))
		}

		_, err = c.etcd.Delete(key, true)
		if err != nil && !strings.HasPrefix(err.Error(), "100:") {
			fmt.Fprintf(os.Stderr, "Error deleting key %s from etcd. Error: %+v\n", key, err)
			c.etcd = nil
		}
	}

	if ok {
		_, err = c.etcd.Set(key, "{}", c.ttl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting key %s to etcd. Error: %+v\n", key, err)
			c.etcd = nil
		}
	}
}

func (c *Containrunner) LoadOrbitConfigurationFromFiles(startpath string) (*OrbitConfiguration, error) {
	oc := new(OrbitConfiguration)
	oc.MachineConfigurations = make(map[string]MachineConfiguration)
	oc.Services = make(map[string]ServiceConfiguration)

	files, err := ioutil.ReadDir(startpath + "/machineconfigurations/tags/")
	if err != nil {
		return nil, err
	}

	for _, tag := range files {
		if !tag.IsDir() {
			panic(errors.New("LoadConfigurationsFromFiles: File " + tag.Name() + " is not a directory!"))
		}

		var mc MachineConfiguration
		var bytes []byte

		fname := startpath + "/machineconfigurations/tags/" + tag.Name() + "/haproxy.tpl"
		bytes, err = ioutil.ReadFile(fname)
		if err == nil {
			mc.HAProxyConfiguration = NewHAProxyConfiguration()
			fmt.Fprintf(os.Stderr, "Loading haproxy config for tag %s from file %s\n", tag.Name(), fname)
			mc.HAProxyConfiguration.Template = string(bytes)
		}

		files, err = ioutil.ReadDir(startpath + "/machineconfigurations/tags/" + tag.Name() + "/haproxy_files/")
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					panic(errors.New("LoadConfigurationsFromFiles: File " + file.Name() + " is a directory when its not allowed"))
				}
				fname := startpath + "/machineconfigurations/tags/" + tag.Name() + "/haproxy_files/" + file.Name()

				fmt.Fprintf(os.Stderr, "Loading haproxy static file %s for tag %s from file %s\n", file.Name(), tag.Name(), fname)

				bytes, err = ioutil.ReadFile(fname)
				if err != nil {
					panic(errors.New(fmt.Sprintf("LoadConfigurationsFromFiles: Could not load static haproxy file %s. Error: %+v", fname, err)))

				}
				mc.HAProxyConfiguration.Files[file.Name()] = string(bytes)

			}
		}

		oc.MachineConfigurations[tag.Name()] = mc

	}

	files, err = ioutil.ReadDir(startpath + "/services/")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		var serviceConfiguration ServiceConfiguration
		fname := startpath + "/services/" + file.Name()
		serviceConfiguration, err = c.ReadServiceFromFile(fname)
		if err != nil {
			return nil, errors.New("LoadConfigurationsFromFiles: Could not load file " + fname)
		}
		fmt.Fprintf(os.Stderr, "Loading service %s from file %s\n", serviceConfiguration.Name, fname)

		oc.Services[serviceConfiguration.Name] = serviceConfiguration
	}

	return oc, nil
}

func (c *Containrunner) UploadOrbitConfigurationToEtcd(orbitConfiguration *OrbitConfiguration, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	for tag, mc := range orbitConfiguration.MachineConfigurations {
		etcdClient.CreateDir(c.EtcdBasePath+"/machineconfigurations/tags/"+tag, 0)

		if mc.HAProxyConfiguration != nil {
			_, err := etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/haproxy_config", mc.HAProxyConfiguration.Template, 0)
			if err != nil {
				return err
			}

			if mc.HAProxyConfiguration.Files != nil {
				for name, contents := range mc.HAProxyConfiguration.Files {
					// TODO: implement old file removal

					_, err := etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/haproxy_files/"+name, contents, 0)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	for name, service := range orbitConfiguration.Services {
		etcdClient.CreateDir(c.EtcdBasePath+"/services/"+name, 0)

		bytes, err := json.Marshal(service)
		_, err = etcdClient.Set(c.EtcdBasePath+"/services/"+name+"/config", string(bytes), 0)
		if err != nil {
			return err
		}

	}

	return nil
}

func (c *Containrunner) GetAllServices(etcdClient *etcd.Client) (map[string]ServiceConfiguration, error) {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	services := make(map[string]ServiceConfiguration)
	var service ServiceConfiguration

	key := c.EtcdBasePath + "/services/"
	res, err := etcdClient.Get(key, true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		return nil, err
	}

	if err != nil {
		return nil, errors.New("No services found. Etcd path was: " + key)
	}

	for _, node := range res.Node.Nodes {
		if node.Dir == true {
			name := string(node.Key[len(res.Node.Key)+1:])
			service, err = c.GetServiceByName(name, etcdClient)
			if err != nil {
				panic(err)
			}
			services[name] = service
		}
	}

	return services, nil
}

func (c *Containrunner) TagServiceToTag(service string, tag string, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	key := c.EtcdBasePath + "/machineconfigurations/tags/" + tag + "/services/" + service
	//fmt.Fprintf(os.Stderr, "Set key: %s\n", key)
	_, err := etcdClient.Set(key, "{}", 0)
	if err != nil {
		return err
	}

	return nil
}

func (c *Containrunner) RemoveService(name string, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	_, err := etcdClient.Delete(c.EtcdBasePath+"/services/"+name, true)
	if err != nil {
		return err
	}

	return nil
}

func (c *Containrunner) GetServiceByName(name string, etcdClient *etcd.Client) (ServiceConfiguration, error) {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	res, err := etcdClient.Get(c.EtcdBasePath+"/services/"+name, true, true)
	if err != nil { // 100: Key not found
		return ServiceConfiguration{}, err
	}

	serviceConfiguration := ServiceConfiguration{}

	for _, node := range res.Node.Nodes {
		if node.Dir == false && strings.HasSuffix(node.Key, "/config") {
			err = json.Unmarshal([]byte(node.Value), &serviceConfiguration)
			if err != nil {
				panic(err)
			}

		}

		/*
			if node.Dir == true && strings.HasSuffix(node.Key, "/endpoints") {
				//			name := node.Key[len(res.Node.Key)+1:]
				//services[name] = c.GetServiceByName(name)
			} */
	}

	return serviceConfiguration, nil
}

func (c *Containrunner) GetKnownTags() ([]string, error) {
	var tags []string
	var etcd *etcd.Client = GetEtcdClient(c.EtcdEndpoints)

	key := c.EtcdBasePath + "/machineconfigurations/tags/"
	res, err := etcd.Get(key, true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		return nil, err
	}

	if err != nil {
		return nil, errors.New("No tags found. Etcd path was: " + key)
	}

	for _, node := range res.Node.Nodes {
		if node.Dir == true {
			name := string(node.Key[len(res.Node.Key)+1:])
			tags = append(tags, name)
		}
	}

	return tags, nil
}

func (c *Containrunner) GetMachineConfigurationByTags(etcd *etcd.Client, tags []string) (MachineConfiguration, error) {

	var configuration MachineConfiguration
	for _, tag := range tags {

		res, err := etcd.Get(c.EtcdBasePath+"/machineconfigurations/tags/"+tag, true, true)
		if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
			panic(err)
		}

		if err != nil {
			continue
		}

		for _, node := range res.Node.Nodes {
			if node.Dir == false && strings.HasSuffix(node.Key, "/authoritative_names") {
				json.Unmarshal([]byte(node.Value), &configuration.AuthoritativeNames)
			}

			if node.Dir == true && strings.HasSuffix(node.Key, "/services") {
				if configuration.Services == nil {
					configuration.Services = make(map[string]ServiceConfiguration, len(node.Nodes))
				}

				for _, service := range node.Nodes {
					if service.Dir == false {
						name := string(service.Key[len(node.Key)+1:])
						service, err := c.GetServiceByName(name, etcd)
						if err != nil {
							return configuration, err
						}
						configuration.Services[name] = service
					}
				}
			}

			if node.Dir == false && strings.HasSuffix(node.Key, "/haproxy_config") {
				if configuration.HAProxyConfiguration == nil {
					configuration.HAProxyConfiguration = NewHAProxyConfiguration()
				}

				configuration.HAProxyConfiguration.Template = node.Value
			}

		}
	}

	return configuration, nil
}

func (c Containrunner) GetHAProxyEndpointsForService(service_name string) (map[string]string, error) {
	etcdClient := GetEtcdClient(c.EtcdEndpoints)

	res, err := etcdClient.Get(c.EtcdBasePath+"/services/"+service_name+"/endpoints", true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		panic(err)
	}

	if err != nil {
		return nil, nil
	}

	backendServers := make(map[string]string)

	for _, endpoint := range res.Node.Nodes {
		if endpoint.Dir == false {
			name := string(endpoint.Key[len(res.Node.Key)+1:])
			backendServers[name] = endpoint.Value
		}
	}

	return backendServers, nil
}

func (c *Containrunner) ReadServiceFromFile(filename string) (ServiceConfiguration, error) {

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	serviceConfiguration := ServiceConfiguration{}
	err = json.Unmarshal(bytes, &serviceConfiguration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading service file %s. Error: %+v\n", filename, err)
		return serviceConfiguration, err
	}

	name := path.Base(filename)[0:strings.Index(path.Base(filename), ".json")]

	if name != serviceConfiguration.Name {
		str := fmt.Sprintf("Integrity mismatch: Different name in filename and inside json: '%s' vs '%s'", name, serviceConfiguration.Name)
		fmt.Fprintf(os.Stderr, "%s\n", str)
		return serviceConfiguration, errors.New(str)
	}

	return serviceConfiguration, nil
}

func (c Containrunner) GetServiceRevision(service_name string, etcd *etcd.Client) (*ServiceRevision, error) {

	res, err := etcd.Get(c.EtcdBasePath+"/services/"+service_name+"/revision", true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		panic(err)
	}

	serviceRevision := new(ServiceRevision)

	err = json.Unmarshal([]byte(res.Node.Value), serviceRevision)
	if err != nil {
		return nil, err
	}

	return serviceRevision, nil
}

func (c Containrunner) SetServiceRevision(service_name string, serviceRevision ServiceRevision, etcd *etcd.Client) error {

	bytes, err := json.Marshal(serviceRevision)
	_, err = etcd.Set(c.EtcdBasePath+"/services/"+service_name+"/revision", string(bytes), 0)
	if err != nil {
		return err
	}

	return nil
}
