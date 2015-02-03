package containrunner

import (
	"encoding/json"
	"fmt"
)

type MissingEtcdPathError struct {
	s string
}

func (e *MissingEtcdPathError) Error() string {
	return e.s
}

type InvalidEtcdConfigFileError struct {
	s string
}

func (e *InvalidEtcdConfigFileError) Error() string {
	return e.s
}

func (c *Containrunner) VerifyAgainstLocalDirectory(directory string) error {

	localoc, err := c.LoadOrbitConfigurationFromFiles(directory)

	if err != nil {
		return err
	}

	return c.VerifyAgainstConfiguration(localoc)
}

func (c *Containrunner) VerifyAgainstConfiguration(localoc *OrbitConfiguration) error {
	etcd := GetEtcdClient(c.EtcdEndpoints)

	if localoc.MachineConfigurations != nil {
		for tagName, machineConfiguration := range localoc.MachineConfigurations {

			path := c.EtcdBasePath + "/machineconfigurations/tags/" + tagName
			_, err := etcd.Get(path, true, true)
			if err != nil {
				return &MissingEtcdPathError{"etcd path missing: " + path}
			}

			if machineConfiguration.Services != nil {
				path := c.EtcdBasePath + "/machineconfigurations/tags/" + tagName + "/services"
				_, err := etcd.Get(path, true, true)
				if err != nil {
					return &MissingEtcdPathError{"etcd path missing: " + path}
				}

				for serviceName, boundServiceConfiguration := range machineConfiguration.Services {
					path := c.EtcdBasePath + "/machineconfigurations/tags/" + tagName + "/services/" + serviceName
					res, err := etcd.Get(path, true, true)
					if err != nil {
						return &MissingEtcdPathError{"etcd path missing: " + path}
					}

					var serviceConfiguration ServiceConfiguration
					if res.Node.Value != "" && res.Node.Value != "{}" {
						err = json.Unmarshal([]byte(res.Node.Value), &serviceConfiguration)
						if err != nil {
							return &InvalidEtcdConfigFileError{"invalid json: " + path}
						}
					}

					if boundServiceConfiguration.Overwrites != nil {
						if DeepEqual(serviceConfiguration, *boundServiceConfiguration.Overwrites) == false {
							fmt.Printf("serviceConfiguration.Container: %+v\n", serviceConfiguration.Container)
							fmt.Printf("          overwrites.Container: %+v\n", boundServiceConfiguration.Overwrites.Container)
							fmt.Printf("tag service overwrite str: %s\n", res.Node.Value)

							return &InvalidEtcdConfigFileError{"invalid content: " + path}
						}
					}

				}
			}

			if machineConfiguration.HAProxyConfiguration != nil {
				path := c.EtcdBasePath + "/machineconfigurations/tags/" + tagName + "/haproxy_config"
				res, err := etcd.Get(path, true, true)
				if err != nil {
					return &MissingEtcdPathError{"etcd path missing: " + path}
				}

				if string(res.Node.Value) != machineConfiguration.HAProxyConfiguration.Template {
					return &InvalidEtcdConfigFileError{"invalid content: " + path}
				}

				// TODO: Check certificates and static files
			}

		}
	}

	if localoc.Services != nil {
		for serviceName, serviceConfig := range localoc.Services {
			path := c.EtcdBasePath + "/services/" + serviceName
			_, err := etcd.Get(path, true, true)
			if err != nil {
				return &MissingEtcdPathError{"etcd path missing: " + path}
			}

			path = c.EtcdBasePath + "/services/" + serviceName + "/config"
			res, err := etcd.Get(path, true, true)
			if err != nil {
				return &MissingEtcdPathError{"etcd path missing: " + path}
			}

			bytes, err := json.Marshal(serviceConfig)
			if res.Node.Value != string(bytes) {
				fmt.Printf("marshalled config: %s\n", bytes)
				fmt.Printf("res.Node   config: %s\n", res.Node.Value)
				return &InvalidEtcdConfigFileError{"invalid content: " + path}
			}
			//c.Assert(res.Node.Value, Equals, `HTTP/1.0 500 Service Unavailable

		}
	}

	fmt.Print("done\n")
	return nil
}
