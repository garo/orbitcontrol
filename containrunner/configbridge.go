package containrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"io/ioutil"
	"os"
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

type MachineConfiguration struct {
	Services             map[string]ServiceConfiguration `json:"services"`
	HAProxyConfiguration *HAProxyConfiguration
	AuthoritativeNames   []string `json:"authoritative_names"`
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
	etcd         *etcd.Client
	ttl          uint64
	EtcdBasePath string
}

// Log events
type ServiceStateChangeEvent struct {
	ServiceName string
	Endpoint    string
	IsUp        bool
}

func (c ConfigResultEtcdPublisher) PublishServiceState(serviceName string, endpoint string, ok bool) {
	key := c.EtcdBasePath + "/services/" + serviceName + "/endpoints/" + endpoint

	_, err := c.etcd.Get(key, false, false)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		panic(err)
	}
	if ok == true && err != nil && strings.HasPrefix(err.Error(), "100:") {
		// Key did not exists so we need to add the key
		log.Info(LogEvent(ServiceStateChangeEvent{serviceName, endpoint, ok}))
	} else if ok == false {
		if err == nil {
			log.Info(LogEvent(ServiceStateChangeEvent{serviceName, endpoint, ok}))
		}

		c.etcd.Delete(key, true)
	}

	if ok {
		c.etcd.Set(key, "{}", c.ttl)
	}
}

func (c *Containrunner) GetAllServices(etcdClient *etcd.Client) (map[string]ServiceConfiguration, error) {
	if etcdClient == nil {
		etcdClient = etcd.NewClient(c.EtcdEndpoints)
	}

	services := make(map[string]ServiceConfiguration)
	var service ServiceConfiguration

	res, err := etcdClient.Get(c.EtcdBasePath+"/services/", true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		return nil, err
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

func (c *Containrunner) RemoveService(name string, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = etcd.NewClient(c.EtcdEndpoints)
	}

	_, err := etcdClient.Delete(c.EtcdBasePath+"/services/"+name, true)
	if err != nil {
		return err
	}

	return nil
}

func (c *Containrunner) GetServiceByName(name string, etcdClient *etcd.Client) (ServiceConfiguration, error) {
	if etcdClient == nil {
		etcdClient = etcd.NewClient(c.EtcdEndpoints)
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
	var etcd *etcd.Client = etcd.NewClient(c.EtcdEndpoints)

	res, err := etcd.Get(c.EtcdBasePath+"/machineconfigurations/tags/", true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		return nil, err
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

		}
	}

	c.GetHAProxyEndpoints(etcd, &configuration)

	return configuration, nil
}

func (c *Containrunner) GetHAProxyEndpoints(etcd *etcd.Client, mc *MachineConfiguration) error {
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

func (c *Containrunner) ImportServiceFromFile(name string, filename string, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = etcd.NewClient(c.EtcdEndpoints)
	}

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	serviceConfiguration := ServiceConfiguration{}
	err = json.Unmarshal(bytes, &serviceConfiguration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading service %s from file %s. Error: %+v\n", name, filename, err)
		return err
	}

	if name != serviceConfiguration.Name {
		str := fmt.Sprintf("Integrity mismatch: Different name in filename and inside json: '%s' vs '%s'", name, serviceConfiguration.Name)
		fmt.Fprintf(os.Stderr, "%s\n", str)
		return errors.New(str)
	}

	bytes, err = json.Marshal(serviceConfiguration)

	_, err = etcdClient.Set(c.EtcdBasePath+"/services/"+name+"/config", string(bytes), 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error importing service %s into etcd. Error: %+v\n", name, err)
		return err
	}

	return nil
}
