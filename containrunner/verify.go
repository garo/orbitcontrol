package containrunner

import (
	"encoding/json"
	"fmt"
	//	"github.com/coreos/go-etcd/etcd"
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
	etcd := GetEtcdClient(c.EtcdEndpoints)

	localoc, err := c.LoadOrbitConfigurationFromFiles(directory)

	if err != nil {
		return err
	}

	for tagName, _ /*machineConfiguration */ := range localoc.MachineConfigurations {

		path := c.EtcdBasePath + "/machineconfigurations/tags/" + tagName
		_, err := etcd.Get(path, true, true)
		if err != nil {
			return &MissingEtcdPathError{"etcd path missing: " + path}
		}

	}

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
			return &InvalidEtcdConfigFileError{"invalid content: " + path}
		}
		//c.Assert(res.Node.Value, Equals, `HTTP/1.0 500 Service Unavailable

	}

	fmt.Print("done\n")
	return err
}
