package containrunner

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/op/go-logging"
	"os"
	"strings"
	"time"
)

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
	var newMachineConfiguration MachineConfiguration
	var err error

	var webserver Webserver
	webserver.Containrunner = &containrunner
	webserver.Start()

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
			newMachineConfiguration, err = containrunner.GetMachineConfigurationByTags(etcdClient, containrunner.Tags)
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
				log.Info(LogString("Sleeping for 5 seconds due to error on GetMachineConfigurationByTags"))
				time.Sleep(time.Second * 5)
				continue
			}

			if !DeepEqual(machineConfiguration, newMachineConfiguration) {

				log.Info(LogString("MainExecutionLoop got new configuration"))

				bytes, _ := json.MarshalIndent(machineConfiguration, "", "    ")
				fmt.Fprintf(os.Stderr, "Old configuration: %s\n", string(bytes))
				bytes, _ = json.MarshalIndent(machineConfiguration, "", "    ")
				fmt.Fprintf(os.Stderr, "New configuration: %s\n", string(bytes))

				go func(containrunner *Containrunner, machineConfiguration MachineConfiguration, oldMachineConfiguration MachineConfiguration) {
					containrunner.HAProxySettings.ConvergeHAProxy(containrunner, machineConfiguration.HAProxyConfiguration, oldMachineConfiguration.HAProxyConfiguration)
				}(&containrunner, newMachineConfiguration, machineConfiguration)

				machineConfiguration = newMachineConfiguration
				ConvergeContainers(machineConfiguration, docker)

				// This must be done after the containers have been converged so that the Check Engine
				// can report the correct container revision
				checkEngine.PushNewConfiguration(machineConfiguration)

				lastConverge = time.Now()
			} else if time.Now().Sub(lastConverge) > time.Second*10 {
				ConvergeContainers(machineConfiguration, docker)
				lastConverge = time.Now()

				//go func(containrunner *Containrunner, machineConfiguration MachineConfiguration) {
				//	containrunner.HAProxySettings.ConvergeHAProxy(containrunner, machineConfiguration.HAProxyConfiguration, nil)
				//}(&containrunner, newMachineConfiguration)
			}

		}

		time.Sleep(time.Second * 2)
		webserver.Keepalive()

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

/*
func LogSocketCount(pos string) {
	out, err := exec.Command("netstat", "-np").Output()
	if err != nil {
		log.Fatal(err)
	}

	m := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Index(line, "orbitctl") != -1 {
			m++
		}
	}
	fmt.Printf("***** %d open sockets at pos %s\n", m, pos)
}*/
