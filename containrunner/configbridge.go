package containrunner

import "github.com/coreos/go-etcd/etcd"
import "strings"
import "encoding/json"

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
	etcd *etcd.Client
	ttl  uint64
}

// Log events
type ServiceStateChangeEvent struct {
	ServiceName string
	Endpoint    string
	IsUp        bool
}

func (c ConfigResultEtcdPublisher) PublishServiceState(serviceName string, endpoint string, ok bool) {
	key := "/services/" + serviceName + "/endpoints/" + endpoint

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

func GetMachineConfigurationByTags(etcd *etcd.Client, tags []string) (MachineConfiguration, error) {

	var configuration MachineConfiguration
	for _, tag := range tags {

		res, err := etcd.Get("/machineconfigurations/tags/"+tag, true, true)
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
						var serviceConfiguration ServiceConfiguration
						err = json.Unmarshal([]byte(service.Value), &serviceConfiguration)
						if err != nil {
							panic(err)
						}

						name := service.Key[len(node.Key)+1:]
						configuration.Services[name] = serviceConfiguration
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

	GetHAProxyEndpoints(etcd, &configuration)

	return configuration, nil
}

func GetHAProxyEndpoints(etcd *etcd.Client, mc *MachineConfiguration) error {
	for serviceName, haProxyEndpoint := range mc.HAProxyConfiguration.Endpoints {

		res, err := etcd.Get("/services/"+serviceName+"/endpoints", true, true)
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
