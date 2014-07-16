package containrunner

import "github.com/coreos/go-etcd/etcd"
import "strings"
import "encoding/json"

type ServiceStateChangeEvent struct {
	ServiceName string
	Endpoint    string
	IsUp        bool
}

type ConfigResultPublisher interface {
	PublishServiceState(serviceName string, endpoint string, ok bool)
}

type ConfigResultEtcdPublisher struct {
	etcd *etcd.Client
	ttl  uint64
}

func (c ConfigResultEtcdPublisher) PublishServiceState(serviceName string, endpoint string, ok bool) {
	key := "/services/" + serviceName + "/" + endpoint

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
				if configuration.Containers == nil {
					configuration.Containers = make(map[string]ContainerConfiguration, len(node.Nodes))
				}

				for _, service := range node.Nodes {
					if service.Dir == false {
						var containerConfiguration ContainerConfiguration
						err = json.Unmarshal([]byte(service.Value), &containerConfiguration)
						if err != nil {
							panic(err)
						}

						name := service.Key[len(node.Key)+1:]
						configuration.Containers[name] = containerConfiguration
					}
				}
			}
		}
	}

	return configuration, nil
}
