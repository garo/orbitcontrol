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
	MachineAddress    string
	CheckIntervalInMs int
	HAProxySettings   HAProxySettings
	EtcdBasePath      string
}

func MainExecutionLoop(exitChannel chan bool, containrunner Containrunner) {

	log.Info(LogString("MainExecutionLoop started"))

	etcdClient := GetEtcdClient(containrunner.EtcdEndpoints)
	docker := GetDockerClient()
	var checkEngine CheckEngine
	checkEngine.Start(4, &ConfigResultEtcdPublisher{10, containrunner.EtcdBasePath, containrunner.EtcdEndpoints, nil}, containrunner.MachineAddress, containrunner.CheckIntervalInMs)

	var machineConfiguration MachineConfiguration

	quit := false
	var lastConverge time.Time
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
			newMachineConfiguration, err := containrunner.GetMachineConfigurationByTags(etcdClient, containrunner.Tags)
			if err != nil {
				if strings.HasPrefix(err.Error(), "100:") {
					log.Info(LogString("Error:" + err.Error()))
				} else if strings.HasPrefix(err.Error(), "50") {
					log.Info(LogString("Error:" + err.Error()))
					log.Info(LogString("Reconnecting to etcd..."))
					etcdClient = GetEtcdClient(containrunner.EtcdEndpoints)

				} else {
					panic(err)
				}
			}
			if !reflect.DeepEqual(machineConfiguration, newMachineConfiguration) {
				log.Info(LogString("MainExecutionLoop got new configuration"))

				go func(containrunner *Containrunner, machineConfiguration MachineConfiguration) {
					containrunner.HAProxySettings.ConvergeHAProxy(machineConfiguration.HAProxyConfiguration)
				}(&containrunner, newMachineConfiguration)

				machineConfiguration = newMachineConfiguration
				checkEngine.PushNewConfiguration(machineConfiguration)
				ConvergeContainers(machineConfiguration, docker)
				lastConverge = time.Now()
			} else if time.Now().Sub(lastConverge) > time.Second*10 {
				ConvergeContainers(machineConfiguration, docker)
				lastConverge = time.Now()

				go func(containrunner *Containrunner, machineConfiguration MachineConfiguration) {
					containrunner.HAProxySettings.ConvergeHAProxy(machineConfiguration.HAProxyConfiguration)
				}(&containrunner, newMachineConfiguration)
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

func GetEtcdClient(endpoints []string) *etcd.Client {
	e := etcd.NewClient(endpoints)
	return e
}
