package containrunner

import (
	"fmt"
	etcd "github.com/coreos/etcd/client"
	"github.com/fsouza/go-dockerclient"
	"github.com/op/go-logging"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var log = logging.MustGetLogger("containrunner")

type Containrunner struct {
	Tags                       []string
	EtcdEndpoints              []string
	exitChannel                chan bool
	MachineAddress             string
	CheckIntervalInMs          int
	HAProxySettings            HAProxySettings
	EtcdBasePath               string
	Events                     MessageQueuer
	Docker                     *docker.Client
	incomingLoopbackEvents     chan OrbitEvent
	DisableAMQP                bool
	NoSleep                    bool
	CommandController          CommandController
	CheckEngine                CheckEngine
	lastConverge               time.Time
	lastConvergeMu             sync.Mutex
	currentConfiguration       RuntimeConfiguration
	newConfiguration           RuntimeConfiguration
	webserver                  Webserver
	pollerStarted              int32
	haproxyUpdateWindowStart   int64
	haproxyUpdateWindowCurrent int64
	HaproxyNoUpdatesDelayWindow int64
	AvailabilityZone           string
}

var configResultPublisher ConfigResultPublisher

type RuntimeConfiguration struct {
	MachineConfiguration MachineConfiguration

	// First string is service name, second string is backend host:port
	ServiceBackends map[string]map[string]*EndpointInfo `json:"-"`

	// Locally required service groups in haproxy, should be refactored away from this struct
	LocallyRequiredServices map[string]map[string]*EndpointInfo `json:"-" DeepEqual:"skip"`
}

func (s *Containrunner) Init() {
	rand.Seed(time.Now().UnixNano())

	etcdClient := GetEtcdClient(s.EtcdEndpoints)

	globalConfiguration, err := s.GetGlobalOrbitProperties(etcdClient)
	if err != nil {
		log.Info(LogString("Could not get global orbit properties"))
		return
	}

	log.Debug("Containrunner.Init called. etcd endpoints: %+v, Global configuration: %+v\n", s.EtcdEndpoints, globalConfiguration)

	var incomingNetworkEvents <-chan OrbitEvent
	s.incomingLoopbackEvents = make(chan OrbitEvent, 1000)
	s.exitChannel = make(chan bool, 1)

	// Check if the message queue features are enabled on this installation
	if globalConfiguration.AMQPUrl != "" && s.DisableAMQP == false {
		log.Debug("Connecting to AMQP: %s\n", globalConfiguration.AMQPUrl)
		s.Events = new(RabbitMQQueuer)

		// Start to listen events on an anonymous queue
		s.Events.Init(globalConfiguration.AMQPUrl, "")
		incomingNetworkEvents = s.Events.GetReceiveredEventChannel()
	}

	s.webserver.Containrunner = s

	s.webserver.Start(1500)

	go s.EventHandler(incomingNetworkEvents, s.incomingLoopbackEvents)

}

// Handles incoming events and calls appropriate event handlers.
//
// There are two different sources for events: Network events and loopback events. The network events
// are delivered via listening a temporary message broker queue. Loopback events
// are simply sent from somewhere in the application instance.
//
// In addition there's a secondary priority block which executes after a short timeout. This
// block polls for new configuration changes and triggers periodic operations.
func (s *Containrunner) EventHandler(incomingNetworkEvents <-chan OrbitEvent, incomingLoopbackEvents <-chan OrbitEvent) {
	etcdClient := GetEtcdClient(s.EtcdEndpoints)

	for {
		select {
		case val, ok := <-s.exitChannel:
			if val == true || ok == false {
				log.Info(LogString("Got exit message"))
				return
			}
			break

		case netEvent, ok := <-incomingNetworkEvents:
			if !ok {
				incomingNetworkEvents = nil
			}
			log.Debug("Got incoming network event: %+v", netEvent)
			s.DispatchEvent(netEvent, etcdClient)
			break

		case loopbackEvent, ok := <-incomingLoopbackEvents:
			if !ok {
				incomingLoopbackEvents = nil
			}
			str := fmt.Sprintf("Got incoming loopback event: %+v", loopbackEvent)
			if len(str) > 200 {
				str = str[0:200]
			}

			log.Debug(str)
			s.DispatchEvent(loopbackEvent, etcdClient)
			break

		case <-time.After(2 * time.Second):
			break
		}

		log.Debug("Items in incomingLoopbackEvents: %d, items in incomingNetworkEvents: %d", len(incomingLoopbackEvents), len(incomingNetworkEvents))

		if atomic.LoadInt32(&s.pollerStarted) == 1 {
			log.Debug("pollerStarted")
			s.webserver.Keepalive()
			// the LastConvergeTime is updated by the HandleConvergeContainerEvent
			if time.Now().Sub(s.GetLastConvergeTime()) > time.Second*10 {

				if !s.CommandController.IsRunning("PollConfigurationUpdate") {
					f := func(arguments interface{}) error {
						log.Debug("Going to push a ConvergeContainerEvent")
						s.PollConfigurationUpdate()
						log.Debug("PollConfigurationUpdate done")

						s.HandleConvergeContainersEvent(ConvergeContainersEvent{s.newConfiguration.MachineConfiguration})
						log.Debug("direct call to HandleConvergeContainersEvent is done")

						return nil
					}
					s.CommandController.InvokeIfNotAlreadyRunning("PollConfigurationUpdate", f, nil)
				}

			}
		}

		// This means that there is a wish to update haproxy in the future
		if atomic.LoadInt64(&s.haproxyUpdateWindowStart) != 0 &&

			// And that the configuration has been stable for 10 seconds
			atomic.LoadInt64(&s.haproxyUpdateWindowCurrent)+s.HaproxyNoUpdatesDelayWindow < time.Now().Unix() {
			log.Info(LogString(fmt.Sprintf("Updating haproxy config as no new configurations have detected in %d seconds after first", s.HaproxyNoUpdatesDelayWindow)))
			conf := NewRuntimeConfigurationEvent{}
			conf.NewRuntimeConfiguration = s.newConfiguration

			s.haproxyUpdateWindowStart = 0
			s.haproxyUpdateWindowCurrent = 0
			go s.ConvergeHAProxy(conf)
		}

		log.Debug("Loop goes around")

		if incomingLoopbackEvents == nil && incomingNetworkEvents == nil {
			return
		}
	}
}

// Takes an OrbitEvent and calls appropriate handler function in a new goroutine.
func (s *Containrunner) DispatchEvent(receiveredEvent OrbitEvent, etcdClient etcd.KeysAPI) {
	switch receiveredEvent.Type {
	case "NoopEvent":
		log.Debug("Got NoopEvent %+v\n", receiveredEvent)
		break
	case "DeploymentEvent":
		//log.Info("Event: %s", receiveredEvent.Type)

		go s.HandleDeploymentEvent(receiveredEvent.Ptr.(DeploymentEvent))
		break
	case "ServiceStateEvent":
		s.HandleServiceStateEvent(receiveredEvent.Ptr.(ServiceStateEvent), etcdClient)
		break
	case "NewRuntimeConfigurationEvent":
		log.Info("Event: %s", receiveredEvent.Type)

		s.HandleNewRuntimeConfigurationEvent(receiveredEvent.Ptr.(NewRuntimeConfigurationEvent))

		break
	case "ConvergeContainersEvent":
		//log.Info("Event: %s", receiveredEvent.Type)
		go s.HandleConvergeContainersEvent(receiveredEvent.Ptr.(ConvergeContainersEvent))
		break
	}
}

func (s *Containrunner) HandleNewRuntimeConfigurationEvent(e NewRuntimeConfigurationEvent) {

	// HAProxy restart grouping feature: When haproxy update is requested it will be delayed for at least 10 seconds
	// to see if there are any other changes coming. In case there are constantly changes coming then restart
	// is forced at 10 second mark.

	// Mark that we want to upgrade haproxy configuration in some point in the near future
	if s.haproxyUpdateWindowStart == 0 {
		s.haproxyUpdateWindowStart = time.Now().Unix()
	}

	// This marks that the configuration has changed recently.
	s.haproxyUpdateWindowCurrent = time.Now().Unix()

	// If haproxy has still not been updated after 60 seconds then force the update
	if s.haproxyUpdateWindowCurrent > s.haproxyUpdateWindowStart+60 {
		s.haproxyUpdateWindowStart = 0
		s.haproxyUpdateWindowCurrent = 0

		log.Info(LogString("Forcing haproxy update 60 seconds after first new update"))
		go s.ConvergeHAProxy(e)
	} else {
		go s.SoftUpdateHAProxy(e)
	}

	s.incomingLoopbackEvents <- NewOrbitEvent(ConvergeContainersEvent{e.NewRuntimeConfiguration.MachineConfiguration})
}

func (s *Containrunner) ConvergeHAProxy(e NewRuntimeConfigurationEvent) {
	if e.NewRuntimeConfiguration.MachineConfiguration.HAProxyConfiguration != nil {
		oldRuntimeConfiguration := &e.OldRuntimeConfiguration
		if !e.OldRuntimeConfigurationValid {
			oldRuntimeConfiguration = nil
		}
		s.HAProxySettings.ConvergeHAProxy(&e.NewRuntimeConfiguration, oldRuntimeConfiguration, s.AvailabilityZone)
	}
}

func (s *Containrunner) SoftUpdateHAProxy(e NewRuntimeConfigurationEvent) {
	if e.NewRuntimeConfiguration.MachineConfiguration.HAProxyConfiguration != nil && e.NewRuntimeConfiguration.LocallyRequiredServices != nil {
		log.Debug("Doing soft haproxy update: %s", e.NewRuntimeConfiguration.LocallyRequiredServices)
		s.HAProxySettings.UpdateBackends(&e.NewRuntimeConfiguration)
	}
}

// Starts ConvergeContainers call. Only one converge process can be executed at a time (enforced via Command subsystem)
// Will discard the event if another converge is already going.
func (s *Containrunner) HandleConvergeContainersEvent(e ConvergeContainersEvent) {

	// Only try to relaunch services which has Container configuration and that the restart command is not already running
	if !s.CommandController.IsRunning("ConvergeContainers") {
		f := func(arguments interface{}) error {
			docker := GetDockerClient()
			configuration := arguments.(MachineConfiguration)
			//log.Info("Converging containers with configuration")
			//log.Info("Converging containers with configuration: %+v", configuration)

			err := ConvergeContainers(configuration, true, !s.NoSleep, docker)

			if err == nil {
				// This must be done after the containers have been converged so that the Check Engine
				// can report the correct container revision

				s.CheckEngine.PushNewConfiguration(configuration)

				s.SetLastConvergeTime(time.Now())
			} else {
				fmt.Printf("Error on ConvergeContainers: %+v\n", err)
			}

			return err
		}
		s.CommandController.InvokeIfNotAlreadyRunning("ConvergeContainers", f, e.MachineConfiguration)
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
	case "AutomaticRelaunch":
		break
	default:
		log.Warning("DeploymentEvent action %s is not implemented", e.Action)

	}
}

func (s *Containrunner) HandleServiceStateEvent(e ServiceStateEvent, etcdClient etcd.KeysAPI) {
	log.Debug("ServiceStateEvent %+v", e)

	if configResultPublisher == nil {
		configResultPublisher = &ConfigResultEtcdPublisher{60, s.EtcdBasePath, s.EtcdEndpoints, etcdClient}
	}

	// Store the availability zone information here
	if e.EndpointInfo != nil {
		e.EndpointInfo.AvailabilityZone = s.AvailabilityZone
	}

	// The etcd result publisher only wants to know when services are up.
	// the TTL feature will automatically kill services which aren't constantly refreshed as
	// being up
	if e.IsUp {
		configResultPublisher.PublishServiceState(e.Service, e.Endpoint, e.IsUp, e.EndpointInfo)
	}

	if e.IsUp == false && time.Since(e.SameStateSince) > time.Minute {
		name := fmt.Sprintf("automatic-relaunch-service-%s", e.Service)

		serviceConfiguration, err := s.GetServiceByName(e.Service, etcdClient, s.MachineAddress)
		if err != nil {
			log.Warning("Error getting service %s configuration from endpoints %+v: %+v", e.Service, s.EtcdEndpoints, err)
			return
		}

		// Only try to relaunch services which has Container configuration and that the restart command is not already running
		if serviceConfiguration.Container != nil && !s.CommandController.IsRunning(name) {
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
				log.Debug("Container %s has been destroyed", name)

				time.Sleep(1 * time.Minute)
				log.Debug("Grace period over for service %s relaunch", name)
				return err
			}
			s.CommandController.InvokeIfNotAlreadyRunning(name, f, e.Service)
		} else {

		}
	}

}

func (s *Containrunner) GetLastConvergeTime() time.Time {
	s.lastConvergeMu.Lock()
	defer s.lastConvergeMu.Unlock()
	return s.lastConverge
}

func (s *Containrunner) SetLastConvergeTime(t time.Time) {
	s.lastConvergeMu.Lock()
	defer s.lastConvergeMu.Unlock()
	s.lastConverge = t
}

func (s *Containrunner) Start() {
	log.Info("Starting check engine with machine address %s", s.MachineAddress)
	s.CheckEngine.Start(4, s.incomingLoopbackEvents, s.MachineAddress, s.CheckIntervalInMs)
	atomic.StoreInt32(&s.pollerStarted, 1)
}

func (s *Containrunner) Wait() {
	<-s.exitChannel
}

func GetEtcdClient(endpoints []string) etcd.KeysAPI {

	cfg := etcd.Config{
		Endpoints:               endpoints,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	client, err := etcd.New(cfg)

	if err != nil {
		panic(err)
	}

	kapi := etcd.NewKeysAPI(client)
	return kapi
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
