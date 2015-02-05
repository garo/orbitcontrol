package containrunner

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/fsouza/go-dockerclient"
	"github.com/op/go-logging"
	"math/rand"
	"os"
	"strings"
	"time"
)

var log = logging.MustGetLogger("containrunner")

type Containrunner struct {
	Tags                   []string
	EtcdEndpoints          []string
	exitChannel            chan bool
	MachineAddress         string
	CheckIntervalInMs      int
	HAProxySettings        HAProxySettings
	EtcdBasePath           string
	Events                 MessageQueuer
	Docker                 *docker.Client
	incomingLoopbackEvents chan OrbitEvent
	DisableAMQP            bool
	CommandController      CommandController
}

type RuntimeConfiguration struct {
	MachineConfiguration MachineConfiguration

	// First string is service name, second string is backend host:port
	ServiceBackends map[string]map[string]*EndpointInfo `json:"-"`

	// Locally required service groups in haproxy, should be refactored away from this struct
	LocallyRequiredServices map[string]map[string]*EndpointInfo `json:"-" DeepEqual:"skip"`
}

func (s *Containrunner) Init() {

	etcdClient := GetEtcdClient(s.EtcdEndpoints)

	globalConfiguration, err := s.GetGlobalOrbitProperties(etcdClient)
	if err != nil {
		log.Info(LogString("Could not get global orbit properties"))
		return
	}

	log.Debug("Containrunner.Init called. etcd endpoints: %+v, Global configuration: %+v\n", s.EtcdEndpoints, globalConfiguration)

	var incomingNetworkEvents <-chan OrbitEvent
	s.incomingLoopbackEvents = make(chan OrbitEvent)

	// Check if the message queue features are enabled on this installation
	if globalConfiguration.AMQPUrl != "" && s.DisableAMQP == false {
		log.Debug("Connecting to AMQP: %s\n", globalConfiguration.AMQPUrl)
		s.Events = new(RabbitMQQueuer)

		// Start to listen events on an anonymous queue
		s.Events.Init(globalConfiguration.AMQPUrl, "")
		incomingNetworkEvents = s.Events.GetReceiveredEventChannel()
	}

	go s.EventHandler(s.incomingLoopbackEvents, incomingNetworkEvents)

}

// Handles incoming events and calls appropriate event handlers.
//
// There are two different sources for events: Network events and loopback events. The network events
// are delivered via listening a temporary message broker queue. Loopback events
// are simply sent from somewhere in the application instance.
func (s *Containrunner) EventHandler(incomingNetworkEvents <-chan OrbitEvent, incomingLoopbackEvents <-chan OrbitEvent) {

	for {
		var receiveredEvent OrbitEvent
		select {
		case event, ok := <-incomingNetworkEvents:
			if !ok {
				incomingNetworkEvents = nil
			}
			receiveredEvent = event
			break
		case event, ok := <-incomingLoopbackEvents:
			if !ok {
				incomingLoopbackEvents = nil
			}
			receiveredEvent = event
			break
		}

		if incomingLoopbackEvents == nil && incomingNetworkEvents == nil {
			return
		}

		switch receiveredEvent.Type {
		case "NoopEvent":
			log.Debug("Got NoopEvent %+v\n", receiveredEvent)
			break
		case "DeploymentEvent":
			go s.HandleDeploymentEvent(receiveredEvent.Ptr.(DeploymentEvent))
			break
		case "ServiceStateEvent":
			go s.HandleServiceStateEvent(receiveredEvent.Ptr.(ServiceStateEvent))
			break
		}
	}
}

func (s *Containrunner) HandleDeploymentEvent(e DeploymentEvent) {
	log.Debug("Got DeploymentEvent %+v\n", e)

	switch e.Action {
	case "RelaunchContainer":
		go func() {
			if e.Jitter > 0 {
				d := rand.Intn(e.Jitter) + 1
				log.Debug("Sleeping %d seconds before destroying old container", d)
				time.Sleep(time.Second * time.Duration(d))
			}
			docker := GetDockerClient()
			err := DestroyContainer(e.Service, docker)
			if err != nil {
				log.Error("Error on RelaunchContainerEvent: %+v\n", err)
			} else {
				log.Debug("Container %s destroyed", e.Service)
			}

		}()

		break
	}

}

var configResultPublisher ConfigResultPublisher

func (s *Containrunner) HandleServiceStateEvent(e ServiceStateEvent) {
	//log.Debug("ServiceStateEvent %+v", e)

	if configResultPublisher != nil {

		// The etcd result publisher only wants to know when services are up.
		// the TTL feature will automatically kill services which aren't constantly refreshed as
		// being up
		if e.IsUp {
			configResultPublisher.PublishServiceState(e.Service, e.Endpoint, e.IsUp, e.EndpointInfo)
		}

		if e.IsUp == false && time.Since(e.SameStateSince) > 20*time.Second {
			name := fmt.Sprintf("automatic-relaunch-service-%s", e.Service)

			if !s.CommandController.IsRunning(name) {
				log.Info("Service %s has been down for too long. Going to proactively relaunch it", e.Service)
				f := func(arguments interface{}) error {
					var name string = arguments.(string)

					log.Debug("Automatic relaunch service will now destroy container %s", name)

					deploymentEvent := DeploymentEvent{}
					deploymentEvent.Action = "AutomaticRelaunch"
					deploymentEvent.Service = name
					deploymentEvent.MachineAddress = s.MachineAddress
					event := NewOrbitEvent(deploymentEvent)
					if s.Events != nil {
						s.Events.PublishOrbitEvent(event)
					}

					docker := GetDockerClient()
					err := DestroyContainer(e.Service, docker)
					if err != nil {
						log.Error("Error destroying container for relaunch: %+v", err)
					}

					time.Sleep(time.Minute)
					log.Debug("Grace period over for service %s relaunch", name)
					return err
				}
				s.CommandController.InvokeIfNotAlreadyRunning(name, f, e.Service)
			}
		}

	} else {
		log.Error("Tried to use nil configResultPublisher!")
	}

}

func MainExecutionLoop(exitChannel chan bool, containrunner Containrunner) {
	rand.Seed(time.Now().UnixNano())
	log.Info(LogString("MainExecutionLoop started"))

	etcdClient := GetEtcdClient(containrunner.EtcdEndpoints)
	docker := GetDockerClient()
	var checkEngine CheckEngine
	configResultPublisher = &ConfigResultEtcdPublisher{10, containrunner.EtcdBasePath, containrunner.EtcdEndpoints, nil}
	checkEngine.Start(4, containrunner.incomingLoopbackEvents, containrunner.MachineAddress, containrunner.CheckIntervalInMs)

	var currentConfiguration RuntimeConfiguration
	var newConfiguration RuntimeConfiguration
	var err error

	var webserver Webserver
	webserver.Containrunner = &containrunner
	//webserver.Start()

	somethingChanged := false

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
			somethingChanged = false

			newConfiguration.MachineConfiguration, err = containrunner.GetMachineConfigurationByTags(etcdClient, containrunner.Tags, containrunner.MachineAddress)
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

			newConfiguration.ServiceBackends, err = containrunner.GetAllServiceEndpoints()

			if !DeepEqual(currentConfiguration.MachineConfiguration, newConfiguration.MachineConfiguration) {
				log.Info(LogString("New Machine Configuration. Pushing changes to check engine"))

				somethingChanged = true
				err := ConvergeContainers(newConfiguration.MachineConfiguration, true, docker)
				if err == nil {
					// This must be done after the containers have been converged so that the Check Engine
					// can report the correct container revision
					checkEngine.PushNewConfiguration(newConfiguration.MachineConfiguration)

					lastConverge = time.Now()
				} else {

					fmt.Printf("Error on ConvergeContainers: %+v\n", err)
				}

			} else if time.Now().Sub(lastConverge) > time.Second*10 {
				ConvergeContainers(newConfiguration.MachineConfiguration, true, docker)
				lastConverge = time.Now()

			}

			if !DeepEqual(currentConfiguration, newConfiguration) && newConfiguration.MachineConfiguration.HAProxyConfiguration != nil {
				somethingChanged = true

				if !DeepEqual(currentConfiguration.MachineConfiguration, newConfiguration.MachineConfiguration) {
					fmt.Fprintf(os.Stderr, "Difference found in MachineConfiguration\n")
					if !DeepEqual(currentConfiguration.MachineConfiguration.HAProxyConfiguration, newConfiguration.MachineConfiguration.HAProxyConfiguration) {
						fmt.Fprintf(os.Stderr, "Difference found in MachineConfiguration.HAProxyConfiguration\n")
					}

					if !DeepEqual(currentConfiguration.MachineConfiguration.Services, newConfiguration.MachineConfiguration.Services) {
						fmt.Fprintf(os.Stderr, "Difference found in MachineConfiguration.Services\n")
					}

				}
				if !DeepEqual(currentConfiguration.ServiceBackends, newConfiguration.ServiceBackends) {
					//fmt.Fprintf(os.Stderr, "Difference found in ServiceBackends\n")

					for service, _ := range currentConfiguration.ServiceBackends {
						_, found := newConfiguration.ServiceBackends[service]
						if found {
							if !DeepEqual(currentConfiguration.ServiceBackends[service], newConfiguration.ServiceBackends[service]) {
								fmt.Fprintf(os.Stderr, "Service %s differs between old and new (%d vs %d items)\n",
									service, len(currentConfiguration.ServiceBackends[service]), len(newConfiguration.ServiceBackends[service]))

								for endpoint, _ := range currentConfiguration.ServiceBackends[service] {
									_, found := newConfiguration.ServiceBackends[service][endpoint]
									if !found {
										fmt.Fprintf(os.Stderr, "Lost endpoint %s from service %s\n", endpoint, service)
									}
								}

								for endpoint, _ := range newConfiguration.ServiceBackends[service] {
									_, found := currentConfiguration.ServiceBackends[service][endpoint]
									if !found {
										fmt.Fprintf(os.Stderr, "New endpoint %s for service %s\n", endpoint, service)
									}
								}

							}
						} else {
							fmt.Fprintf(os.Stderr, "Service %s not found in new ServiceBackends\n", service)
						}
					}
				}

				//bytes, _ := json.MarshalIndent(currentConfiguration, "", "    ")
				//fmt.Fprintf(os.Stderr, "Old configuration: %s\n", string(bytes))
				//bytes, _ = json.MarshalIndent(newConfiguration, "", "    ")
				//fmt.Fprintf(os.Stderr, "New configuration: %s\n", string(bytes))

				go func(containrunner *Containrunner, runtimeConfiguration RuntimeConfiguration, oldConfiguration RuntimeConfiguration) {
					containrunner.HAProxySettings.ConvergeHAProxy(&runtimeConfiguration, &oldConfiguration)
				}(&containrunner, newConfiguration, currentConfiguration)

			}

			if somethingChanged {
				currentConfiguration = newConfiguration
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
