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
	/orbit/services/<name>/endpoints/<endpoint host:port>
	/orbit/machineconfigurations/tags/<tag>/authoritative_names
	/orbit/machineconfigurations/tags/<tag>/services/<service_name>
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

	for _, file := range files {
		if !file.IsDir() {
			panic(errors.New("LoadConfigurationsFromFiles: File " + file.Name() + " is not a directory!"))
		}

		var mc MachineConfiguration
		var bytes []byte

		var haproxy_endpoint_files []os.FileInfo
		haproxy_endpoint_files, err = ioutil.ReadDir(startpath + "/machineconfigurations/tags/" + file.Name() + "/haproxy_endpoints")
		if err == nil {
			mc.HAProxyConfiguration = NewHAProxyConfiguration()
			fname := startpath + "/machineconfigurations/tags/" + file.Name() + "/haproxy.json"
			bytes, err = ioutil.ReadFile(fname)
			if err == nil {
				fmt.Fprintf(os.Stderr, "Loading haproxy config for tag %s from file %s\n", file.Name(), fname)
				err = json.Unmarshal(bytes, mc.HAProxyConfiguration)
				if err != nil {
					return nil, errors.New(fmt.Sprintf("Error loading file %s. Error: %+v\n", fname, err))
				}
			}

			for _, haproxy_endpoint_file := range haproxy_endpoint_files {
				fname = startpath + "/machineconfigurations/tags/" + file.Name() + "/haproxy_endpoints/" + haproxy_endpoint_file.Name()
				bytes, err = ioutil.ReadFile(fname)
				endpoint := NewHAProxyEndpoint()
				err = json.Unmarshal(bytes, endpoint)
				fmt.Fprintf(os.Stderr, "Loading haproxy endpoint %s for tag %s\n", endpoint.Name, file.Name())

				if err != nil {
					return nil, errors.New(fmt.Sprintf("Error loading file %s. Error: %+v\n", fname, err))
				}
				mc.HAProxyConfiguration.Endpoints[endpoint.Name] = endpoint
			}
		}

		oc.MachineConfigurations[file.Name()] = mc

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
			bytes, err := json.Marshal(mc.HAProxyConfiguration)
			_, err = etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/haproxy_config", string(bytes), 0)
			if err != nil {
				return err
			}

			etcdClient.CreateDir(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/haproxy_endpoints", 0)
			for name, haproxy_endpoint := range mc.HAProxyConfiguration.Endpoints {
				bytes, err = json.Marshal(haproxy_endpoint)
				_, err = etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/haproxy_endpoints/"+name, string(bytes), 0)
				if err != nil {
					return err
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
			name := node.Key[len(res.Node.Key)+1:]
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
			name := node.Key[len(res.Node.Key)+1:]
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
						name := service.Key[len(node.Key)+1:]
						service, err := c.GetServiceByName(name, etcd)
						if err != nil {
							return configuration, err
						}
						configuration.Services[name] = service
					}
				}
			}

			if node.Dir == true && strings.HasSuffix(node.Key, "/haproxy_endpoints") {
				if configuration.HAProxyConfiguration == nil {
					configuration.HAProxyConfiguration = NewHAProxyConfiguration()
				}

				for _, haProxyEndpoint := range node.Nodes {
					if haProxyEndpoint.Dir == false {
						endpoint := new(HAProxyEndpoint)
						err = json.Unmarshal([]byte(haProxyEndpoint.Value), &endpoint)
						if err != nil {
							panic(err)
						}

						name := haProxyEndpoint.Key[len(node.Key)+1:]
						configuration.HAProxyConfiguration.Endpoints[name] = endpoint
					}
				}
			}

			if node.Dir == false && strings.HasSuffix(node.Key, "/haproxy_config") {
				if configuration.HAProxyConfiguration == nil {
					configuration.HAProxyConfiguration = NewHAProxyConfiguration()
				}

				err = json.Unmarshal([]byte(node.Value), configuration.HAProxyConfiguration)
				if err != nil {
					panic(err)
				}
			}

		}
	}

	c.GetHAProxyEndpoints(etcd, &configuration)

	return configuration, nil
}

func (c *Containrunner) GetHAProxyEndpoints(etcd *etcd.Client, mc *MachineConfiguration) error {
	if mc.HAProxyConfiguration == nil {
		return nil
	}

	for serviceName, haProxyEndpoint := range mc.HAProxyConfiguration.Endpoints {

		res, err := etcd.Get(c.EtcdBasePath+"/services/"+serviceName+"/endpoints", true, true)
		if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
			panic(err)
		}

		if err != nil {
			continue
		}

		if haProxyEndpoint.BackendServers == nil {
			haProxyEndpoint.BackendServers = make(map[string]string)
		}

		for _, endpoint := range res.Node.Nodes {
			if endpoint.Dir == false {
				name := endpoint.Key[len(res.Node.Key)+1:]
				haProxyEndpoint.BackendServers[name] = endpoint.Value
			}
		}
	}

	return nil
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
