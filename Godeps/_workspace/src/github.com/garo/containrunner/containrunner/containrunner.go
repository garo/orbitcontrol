package containrunner

import "github.com/coreos/go-etcd/etcd"
import "time"
import "github.com/op/go-logging"
import "strings"
import "reflect"

var log = logging.MustGetLogger("containrunner")

type Containrunner struct {
	Tags              []string
	EtcdEndpoints     []string
	exitChannel       chan bool
	EndpointAddress   string
	CheckIntervalInMs int
	HAProxySettings   HAProxySettings
}

func MainExecutionLoop(exitChannel chan bool, containrunner Containrunner) {

	log.Info(LogString("MainExecutionLoop started"))

	etcd := etcd.NewClient(containrunner.EtcdEndpoints)
	docker := GetDockerClient()
	var checkEngine CheckEngine
	checkEngine.Start(4, ConfigResultEtcdPublisher{etcd, 10}, containrunner.EndpointAddress, containrunner.CheckIntervalInMs)

	var machineConfiguration MachineConfiguration

	quit := false
	for !quit {
		select {
		case val := <-exitChannel:
			if val == true {
				log.Info(LogString("MainExecutionLoop stopping"))
				quit = true
				checkEngine.Stop()
				//etcd.Close()
				//docker.Close()
				exitChannel <- true
			}
		default:
			newMachineConfiguration, err := GetMachineConfigurationByTags(etcd, containrunner.Tags)
			if err != nil && !strings.HasPrefix(err.Error(), "100:") {
				log.Info(LogString("Error:" + err.Error()))
				panic(err)
			}
			if !reflect.DeepEqual(machineConfiguration, newMachineConfiguration) {
				log.Info(LogString("MainExecutionLoop got new configuration"))

				go func(containrunner *Containrunner, machineConfiguration MachineConfiguration) {
					containrunner.HAProxySettings.ConvergeHAProxy(machineConfiguration.HAProxyConfiguration)
				}(&containrunner, machineConfiguration)

				machineConfiguration = newMachineConfiguration
				checkEngine.PushNewConfiguration(machineConfiguration)
				ConvergeContainers(machineConfiguration, docker)
			}

		}

		time.Sleep(time.Second * 2)

	}
}

func (s *Containrunner) Start() {
	log.Info(LogString("Starting Containrunner..."))

	s.exitChannel = make(chan bool, 1)

	go MainExecutionLoop(s.exitChannel, *s)
}

func (s *Containrunner) Wait() {
	<-s.exitChannel
}
