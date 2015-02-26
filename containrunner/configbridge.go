package containrunner

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

/*
	Data model referred
	/orbit/env <-- file contents tells the environment
	/orbit/globalproperties				// JSON file containing GlobalOrbitProperties data
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
	GlobalOrbitProperties GlobalOrbitProperties
}

type GlobalOrbitProperties struct {
	AMQPUrl string
}

// Represents a single tag inside a /orbit/machineconfiguration/
type TagConfiguration struct {
	Services             map[string]BoundService `json:"services"`
	HAProxyConfiguration *HAProxyConfiguration
	AuthoritativeNames   []string `json:"authoritative_names"`
}

// Represents all configurations for a single physical machine.
// Because all tags are unioned into one this can be represented
// with just a single unioned TagConfiguration but its kept as a separated
// type for now
type MachineConfiguration struct {
	TagConfiguration
}

// This defines a service which has been bound to a machine with a tag
// A bound service can overwrite its default service configuration.
//
// The DefaultConfiguration is a pointer to OrbitConfiguration.Services[name]
// and the actual runtime configuration is created by merging these two together.
type BoundService struct {
	DefaultConfiguration ServiceConfiguration
	Overwrites           *ServiceConfiguration

	cachedMergedConfig *ServiceConfiguration
}

// This represents a default configuration for a single service which is global
// for the entire Orbit deployment. These services can be bound into a set of
// machines using tags and this bind is represented with the BoundService structure above.
type ServiceConfiguration struct {
	Name          string
	EndpointPort  int
	Checks        []ServiceCheck
	Container     *ContainerConfiguration
	Revision      *ServiceRevision
	SourceControl *SourceControl
	Attributes    map[string]string
}

type SourceControl struct {
	Origin     string
	OAuthToken string
	CIUrl      string
}

type ServiceRevision struct {
	Revision       string
	DeploymentTime time.Time
}

type ConfigResultPublisher interface {
	PublishServiceState(serviceName string, endpoint string, ok bool, info *EndpointInfo)
}

type ConfigResultEtcdPublisher struct {
	ttl           uint64
	EtcdBasePath  string
	EtcdEndpoints []string
	etcd          *etcd.Client
}

// Stored inside file /orbit/services/<service>/endpoints/<host:port>
type EndpointInfo struct {
	Revision             string
	ServiceConfiguration ServiceConfiguration
}

// Log events
type ServiceStateChangeEvent struct {
	ServiceName string
	Endpoint    string
	IsUp        bool
}

func NewMachineConfiguration() *MachineConfiguration {
	mc := new(MachineConfiguration)
	mc.Services = make(map[string]BoundService)

	return mc
}

func (c BoundService) GetConfig() ServiceConfiguration {
	if c.cachedMergedConfig != nil {
		return *(c.cachedMergedConfig)
	}

	if c.Overwrites != nil {
		tmp := MergeServiceConfig(c.DefaultConfiguration, *c.Overwrites)
		c.cachedMergedConfig = &tmp
		return *(c.cachedMergedConfig)
	}

	return c.DefaultConfiguration
}

func (c *ConfigResultEtcdPublisher) PublishServiceState(serviceName string, endpoint string, ok bool, info *EndpointInfo) {
	if c.etcd == nil {
		log.Debug("Creating new Etcd client (%+v) so that we can report that service %s at %s is %+v using ttl %d\n", c.EtcdEndpoints, serviceName, endpoint, ok, c.ttl)
		c.etcd = GetEtcdClient(c.EtcdEndpoints)

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
		bytes, err := json.Marshal(info)
		_, err = c.etcd.Set(key, string(bytes), c.ttl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting key %s to etcd. Error: %+v\n", key, err)
			c.etcd = nil
		}
	}
}

// Polls for configuration updates and triggers a NewRuntimeConfigurationEvent.
func (s *Containrunner) PollConfigurationUpdate() {
	etcdClient := GetEtcdClient(s.EtcdEndpoints)

	var err error

	// Handle new MachineConfiguration
	s.newConfiguration.MachineConfiguration, err = s.GetMachineConfigurationByTags(etcdClient, s.Tags, s.MachineAddress)
	if err != nil {
		if strings.HasPrefix(err.Error(), "100:") {
			log.Info(LogString("Error:" + err.Error()))
		} else if strings.HasPrefix(err.Error(), "50") {
			log.Info(LogString("Error:" + err.Error()))
			log.Info(LogString("Reconnecting to etcd..."))
			etcdClient = GetEtcdClient(s.EtcdEndpoints)

		} else {
			panic(err)
		}
		return
	}

	// Handle new ServiceBackends
	backends, err := GetAllServiceEndpoints(s.EtcdEndpoints, s.EtcdBasePath)
	s.newConfiguration.ServiceBackends = backends
	if err != nil {
		log.Error("GetAllServiceEndpoints error: %+v", err)
		return
	}

	if !DeepEqual(s.currentConfiguration, s.newConfiguration) {
		event := NewRuntimeConfigurationEvent{}
		event.OldRuntimeConfiguration = s.currentConfiguration
		event.NewRuntimeConfiguration = s.newConfiguration

		log.Info("Going to send NewRuntimeConfigurationEvent")
		s.incomingLoopbackEvents <- NewOrbitEvent(event)

		s.currentConfiguration = s.newConfiguration
	}
}

func (c *Containrunner) LoadOrbitConfigurationFromFiles(startpath string) (*OrbitConfiguration, error) {
	oc := new(OrbitConfiguration)
	oc.MachineConfigurations = make(map[string]MachineConfiguration)
	oc.Services = make(map[string]ServiceConfiguration)

	file := startpath + "/globalproperties.json"
	if _, err := os.Stat(file); err == nil {
		bytes, err := ioutil.ReadFile(file)
		err = json.Unmarshal(bytes, &oc.GlobalOrbitProperties)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("LoadConfigurationsFromFiles: Could not load globalproperties.json due to error %+v", err))
		} else {
			fmt.Printf("Read globalproperties from %s\n", file)
		}
	}

	files, err := ioutil.ReadDir(startpath + "/services/")
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

	files, err = ioutil.ReadDir(startpath + "/machineconfigurations/tags/")
	if err != nil {
		return nil, err
	}

	for _, tag := range files {
		if !tag.IsDir() {
			panic(errors.New("LoadConfigurationsFromFiles: File " + tag.Name() + " is not a directory!"))
		}

		var mc MachineConfiguration
		mc.Services = make(map[string]BoundService)
		var bytes []byte

		fname := startpath + "/machineconfigurations/tags/" + tag.Name() + "/haproxy.tpl"
		bytes, err = ioutil.ReadFile(fname)
		if err == nil {
			mc.HAProxyConfiguration = NewHAProxyConfiguration()
			fmt.Fprintf(os.Stderr, "Loading haproxy config for tag %s from file ..%s\n", tag.Name(), fname[len(startpath):])
			mc.HAProxyConfiguration.Template = string(bytes)
		}

		fname = startpath + "/machineconfigurations/tags/" + tag.Name() + "/authoritative_names.json"
		bytes, err = ioutil.ReadFile(fname)
		if err == nil {
			err = json.Unmarshal([]byte(bytes), &mc.AuthoritativeNames)
		}

		files, err = ioutil.ReadDir(startpath + "/machineconfigurations/tags/" + tag.Name() + "/certs/")
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					panic(errors.New("LoadConfigurationsFromFiles: File " + file.Name() + " is a directory when its not allowed"))
				}
				fname := startpath + "/machineconfigurations/tags/" + tag.Name() + "/certs/" + file.Name()

				fmt.Fprintf(os.Stderr, "Loading certificate %s for tag %s from file ..%s\n", file.Name(), tag.Name(), fname[len(startpath):])

				bytes, err = ioutil.ReadFile(fname)
				if err != nil {
					panic(errors.New(fmt.Sprintf("LoadConfigurationsFromFiles: Could not load certificate file %s. Error: %+v", fname, err)))

				}
				if mc.HAProxyConfiguration == nil {
					panic(errors.New("LoadConfigurationsFromFiles: There were certificates but no haproxy.tpl"))
				} else {
					mc.HAProxyConfiguration.Certs[file.Name()] = string(bytes)
				}

			}
		}

		files, err = ioutil.ReadDir(startpath + "/machineconfigurations/tags/" + tag.Name() + "/haproxy_files/")
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					panic(errors.New("LoadConfigurationsFromFiles: File " + file.Name() + " is a directory when its not allowed"))
				}
				fname := startpath + "/machineconfigurations/tags/" + tag.Name() + "/haproxy_files/" + file.Name()

				fmt.Fprintf(os.Stderr, "Loading haproxy static file %s for tag %s from file ..%s\n", file.Name(), tag.Name(), fname[len(startpath):])

				bytes, err = ioutil.ReadFile(fname)
				if err != nil {
					panic(errors.New(fmt.Sprintf("LoadConfigurationsFromFiles: Could not load static haproxy file %s. Error: %+v", fname, err)))

				}
				mc.HAProxyConfiguration.Files[file.Name()] = string(bytes)

			}
		}

		files, err = ioutil.ReadDir(startpath + "/machineconfigurations/tags/" + tag.Name() + "/services/")
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					panic(errors.New("LoadConfigurationsFromFiles: File " + file.Name() + " is a directory when its not allowed"))
				}
				fname := startpath + "/machineconfigurations/tags/" + tag.Name() + "/services/" + file.Name()
				service_name := file.Name()[0 : len(file.Name())-5]

				boundService := BoundService{}

				fmt.Fprintf(os.Stderr, "Loading service %s from tag mapping file %s for tag %s from file ..%s\n", service_name, file.Name(), tag.Name(), fname[len(startpath):])

				bytes, err = ioutil.ReadFile(fname)
				if err != nil {
					panic(errors.New(fmt.Sprintf("LoadConfigurationsFromFiles: Could not load tagging file %s. Error: %+v", fname, err)))
				}
				str := string(bytes)
				if str != "" && str != "{}" {
					boundService.Overwrites = &ServiceConfiguration{}
					err = json.Unmarshal([]byte(bytes), boundService.Overwrites)
				}

				service, ok := oc.Services[service_name]
				if !ok {
					fmt.Fprintf(os.Stderr, "Could not find service %s when tried to tag it to %s\n", service_name, tag.Name())
					os.Exit(1)
					return nil, err
				}
				boundService.DefaultConfiguration = service

				mc.Services[service_name] = boundService

			}
		}

		oc.MachineConfigurations[tag.Name()] = mc

	}

	return oc, nil
}

func (c *Containrunner) UploadOrbitConfigurationToEtcd(orbitConfiguration *OrbitConfiguration, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
		fmt.Fprintf(os.Stderr, "EtcdEndpoints: %s\n", c.EtcdEndpoints)
	}

	bytes, err := json.Marshal(orbitConfiguration.GlobalOrbitProperties)
	_, err = etcdClient.Set(c.EtcdBasePath+"/globalproperties", string(bytes), 0)
	if err != nil {
		return err
	}

	for tag, mc := range orbitConfiguration.MachineConfigurations {
		etcdClient.CreateDir(c.EtcdBasePath+"/machineconfigurations/tags/"+tag, 0)
		etcdClient.CreateDir(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/services", 0)

		if mc.HAProxyConfiguration != nil {
			_, err := etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/haproxy_config", mc.HAProxyConfiguration.Template, 0)
			if err != nil {
				return err
			}

			if mc.HAProxyConfiguration.Certs != nil {
				for name, contents := range mc.HAProxyConfiguration.Certs {
					// TODO: implement old file removal

					_, err := etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/certs/"+name, contents, 0)
					if err != nil {
						return err
					}
				}
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

		if len(mc.AuthoritativeNames) > 0 {
			bytes, err := json.Marshal(mc.AuthoritativeNames)
			_, err = etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/authoritative_names", string(bytes), 0)
			if err != nil {
				return err
			}

		}

		// First check if a service needs to be removed
		key := c.EtcdBasePath + "/machineconfigurations/tags/" + tag + "/services"
		res, err := etcdClient.Get(key, true, true)
		if err == nil {
			for _, node := range res.Node.Nodes {
				name := string(node.Key[len(res.Node.Key)+1:])
				_, exists := mc.Services[name]
				if exists == false {
					fmt.Printf("Service %s does not exists any more in tag %s, deleting it.\n", name, tag)
					etcdClient.Delete(node.Key, true)
				}
			}
		}

		for name, boundService := range mc.Services {
			str := "{}"
			if boundService.Overwrites != nil {
				bytes, err := json.Marshal(boundService.Overwrites)
				if err != nil {
					return err
				}
				str = string(bytes)
			}

			_, err := etcdClient.Set(c.EtcdBasePath+"/machineconfigurations/tags/"+tag+"/services/"+name, str, 0)
			if err != nil {
				return err
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
			service, err = c.GetServiceByName(name, etcdClient, "")
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

func (c *Containrunner) GetServiceByName(name string, etcdClient *etcd.Client, machineAddress string) (ServiceConfiguration, error) {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	res, err := etcdClient.Get(c.EtcdBasePath+"/services/"+name, true, true)
	if err != nil { // 100: Key not found
		return ServiceConfiguration{}, err
	}

	serviceConfiguration := ServiceConfiguration{}
	var serviceRevision *ServiceRevision = nil
	//var machineServiceRevision *ServiceRevision

	for _, node := range res.Node.Nodes {
		if node.Dir == false && strings.HasSuffix(node.Key, "/config") {
			err = json.Unmarshal([]byte(node.Value), &serviceConfiguration)
			if err != nil {
				panic(err)
			}
		}
		if node.Dir == true && strings.HasSuffix(node.Key, "/machines") {
			for _, subnode := range node.Nodes {
				ip := subnode.Key[len(node.Key)+1:]
				if ip == machineAddress {
					tmp := new(ServiceRevision)
					err = json.Unmarshal([]byte(subnode.Value), tmp)
					if err != nil {
						fmt.Printf("Error unmarshalling machine revision. Error: %+v\n. String data was: %s\n", err, subnode.Value)
					} else {
						serviceRevision = tmp
					}

				}
			}
		}

		if node.Dir == false && strings.HasSuffix(node.Key, "/revision") {

			// We might have a machine revision which overwrites the default revision. If so, dont unmarshall the default revision here
			if serviceRevision == nil {
				serviceRevision = new(ServiceRevision)
				err = json.Unmarshal([]byte(node.Value), serviceRevision)
				if err != nil {
					panic(err)
				}
			}
		}

		/*
			if node.Dir == true && strings.HasSuffix(node.Key, "/endpoints") {
				//			name := node.Key[len(res.Node.Key)+1:]
				//services[name] = c.GetServiceByName(name)
			} */
	}

	if serviceRevision != nil {
		serviceConfiguration.Revision = serviceRevision
	}

	return serviceConfiguration, nil
}

func (c *Containrunner) GetGlobalOrbitProperties(etcdClient *etcd.Client) (GlobalOrbitProperties, error) {
	gop := GlobalOrbitProperties{}

	key := c.EtcdBasePath + "/globalproperties"
	res, err := etcdClient.Get(key, true, true)

	if err != nil {
		if !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
			return gop, err
		} else {
			return gop, nil
		}
	}

	err = json.Unmarshal([]byte(res.Node.Value), &gop)

	return gop, err
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

func CopyServiceConfiguration(src ServiceConfiguration) ServiceConfiguration {
	var network bytes.Buffer
	enc := gob.NewEncoder(&network)
	dec := gob.NewDecoder(&network)

	err := enc.Encode(src)
	if err != nil {
		panic("Error copying structure")
	}

	var dst ServiceConfiguration
	err = dec.Decode(&dst)
	if err != nil {
		panic("Error on decode structure")
	}

	return dst
}

func MergeServiceConfig(dst ServiceConfiguration, overwrite ServiceConfiguration) ServiceConfiguration {

	dst = CopyServiceConfiguration(dst)
	overwrite = CopyServiceConfiguration(overwrite)

	if overwrite.EndpointPort != 0 {
		dst.EndpointPort = overwrite.EndpointPort
	}

	if overwrite.Container != nil {
		if &overwrite.Container.Config != nil {
			if &overwrite.Container.Config.Env != nil {
				if &dst.Container.Config.Env == nil {
					dst.Container.Config.Env = overwrite.Container.Config.Env
				} else {
					env := make(map[string]string)
					for _, e := range dst.Container.Config.Env {
						parts := strings.Split(e, "=")
						env[parts[0]] = parts[1]
					}

					for _, e := range overwrite.Container.Config.Env {
						parts := strings.Split(e, "=")
						env[parts[0]] = parts[1]
					}

					dst.Container.Config.Env = make([]string, len(env))
					i := 0
					for k, v := range env {
						dst.Container.Config.Env[i] = (k + "=" + v)
						i++
					}

					sort.Strings(dst.Container.Config.Env)

				}
			}

			if overwrite.Container.Config.Image != "" {
				dst.Container.Config.Image = overwrite.Container.Config.Image
			}

			if overwrite.Container.Config.Hostname != "" {
				dst.Container.Config.Hostname = overwrite.Container.Config.Hostname
			}

		}
	}

	// If any check is defined in the overwrite settings then the entire overwrite checks rule array
	// will overwrite the default. ie. we don't try to merge these together.
	if overwrite.Checks != nil {
		dst.Checks = overwrite.Checks
	}

	if overwrite.Attributes != nil {
		if dst.Attributes == nil {
			dst.Attributes = overwrite.Attributes
		} else {
			for key, value := range overwrite.Attributes {
				dst.Attributes[key] = value
			}
		}
	}

	return dst
}

func (c *Containrunner) GetMachineConfigurationByTags(etcd *etcd.Client, tags []string, machineAddress string) (MachineConfiguration, error) {

	var configuration MachineConfiguration
	for _, tag := range tags {

		key := c.EtcdBasePath + "/machineconfigurations/tags/" + tag
		res, err := etcd.Get(key, true, true)
		if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
			fmt.Fprintf(os.Stderr, "Error getting machine configuration from key %s. Err: %+v\n", key, err)
			return configuration, err
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
					configuration.Services = make(map[string]BoundService, len(node.Nodes))
				}

				for _, serviceNode := range node.Nodes {
					if serviceNode.Dir == false {
						name := string(serviceNode.Key[len(node.Key)+1:])

						boundService := BoundService{}

						// The GetServiceByName creates completly new ServiceConfiguration instance
						// So it's later safe to use MergeServiceConfig to modify it (it's not shared between machines or anything)
						boundService.DefaultConfiguration, err = c.GetServiceByName(name, etcd, machineAddress)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error getting service %s: %+v\n", name, err)
							return configuration, err
						}

						if serviceNode.Value != "" && serviceNode.Value != "{}" {
							var overwrite ServiceConfiguration
							err = json.Unmarshal([]byte(serviceNode.Value), &overwrite)

							boundService.Overwrites = &overwrite
						}
						configuration.Services[name] = boundService
					}
				}
			}

			if node.Dir == false && strings.HasSuffix(node.Key, "/haproxy_config") {
				if configuration.HAProxyConfiguration == nil {
					configuration.HAProxyConfiguration = NewHAProxyConfiguration()
				}

				configuration.HAProxyConfiguration.Template = node.Value
			}

			if node.Dir == true && strings.HasSuffix(node.Key, "/certs") {
				if configuration.HAProxyConfiguration == nil {
					configuration.HAProxyConfiguration = NewHAProxyConfiguration()
				}

				for _, file := range node.Nodes {
					if file.Dir == false {
						name := string(file.Key[len(node.Key)+1:])
						configuration.HAProxyConfiguration.Certs[name] = file.Value
					}
				}
			}

			if node.Dir == true && strings.HasSuffix(node.Key, "/haproxy_files") {
				if configuration.HAProxyConfiguration == nil {
					configuration.HAProxyConfiguration = NewHAProxyConfiguration()
				}

				for _, file := range node.Nodes {
					if file.Dir == false {
						name := string(file.Key[len(node.Key)+1:])
						configuration.HAProxyConfiguration.Files[name] = file.Value
					}
				}
			}

		}
	}

	return configuration, nil
}

func GetAllServiceEndpoints(etcdEndpoints []string, etcdBasePath string) (map[string]map[string]*EndpointInfo, error) {

	serviceBackends := make(map[string]map[string]*EndpointInfo)

	etcdClient := GetEtcdClient(etcdEndpoints)
	defer func() {
		etcdClient.CloseCURL()
	}()

	res, err := etcdClient.Get(etcdBasePath+"/services/", true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		log.Error("Could not get services from etcd endpoint: %+v", etcdEndpoints)
		panic(err)
	}

	if err != nil {
		return nil, nil
	}

	// Iterate over all services...
	for _, service := range res.Node.Nodes {

		// By finding the directory which name is the service name
		if service.Dir == true {
			serviceName := string(service.Key[len(res.Node.Key)+1:])

			// From that directory iterate over all items
			for _, item := range service.Nodes {
				itemName := string(item.Key[len(service.Key)+1:])

				// Until you find the "endpoints" item which must be a directory
				if itemName == "endpoints" {

					// Then iterate over all files in that directory
					for _, endpoint := range item.Nodes {
						endpointName := string(endpoint.Key[len(item.Key)+1:])

						// Which are marshalled EndpointInfo objects
						endpointInfo := new(EndpointInfo)
						json.Unmarshal([]byte(endpoint.Value), endpointInfo)

						if _, found := serviceBackends[serviceName]; found == false {
							serviceBackends[serviceName] = make(map[string]*EndpointInfo)
						}

						serviceBackends[serviceName][endpointName] = endpointInfo
					}
				}
			}
		}
	}

	return serviceBackends, nil
}

func (c Containrunner) GetEndpointsForService(service_name string) (map[string]*EndpointInfo, error) {

	etcdClient := GetEtcdClient(c.EtcdEndpoints)
	defer func() {
		etcdClient.CloseCURL()
	}()

	res, err := etcdClient.Get(c.EtcdBasePath+"/services/"+service_name+"/endpoints", true, true)
	if err != nil && !strings.HasPrefix(err.Error(), "100:") { // 100: Key not found
		log.Error("Could not get service endpoints: %+v", err.Error())
		return nil, err
	}

	if err != nil {
		return nil, nil
	}

	backendServers := make(map[string]*EndpointInfo)

	for _, endpoint := range res.Node.Nodes {
		if endpoint.Dir == false {
			name := string(endpoint.Key[len(res.Node.Key)+1:])
			endpointInfo := new(EndpointInfo)
			json.Unmarshal([]byte(endpoint.Value), endpointInfo)
			backendServers[name] = endpointInfo
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

func (c Containrunner) SetServiceRevision(service_name string, serviceRevision ServiceRevision, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	bytes, err := json.Marshal(serviceRevision)
	_, err = etcdClient.Set(c.EtcdBasePath+"/services/"+service_name+"/revision", string(bytes), 0)
	if err != nil {
		return err
	}

	etcdClient.Delete(c.EtcdBasePath+"/services/"+service_name+"/machines", true)

	return nil
}

func (c Containrunner) SetServiceRevisionForMachine(service_name string, serviceRevision ServiceRevision, machineAddress string, etcdClient *etcd.Client) error {
	if etcdClient == nil {
		etcdClient = GetEtcdClient(c.EtcdEndpoints)
	}

	bytes, err := json.Marshal(serviceRevision)
	_, err = etcdClient.Set(c.EtcdBasePath+"/services/"+service_name+"/machines/"+machineAddress, string(bytes), 0)
	if err != nil {
		return err
	}

	return nil
}
