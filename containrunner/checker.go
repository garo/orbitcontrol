package containrunner

import "net"
import "time"

import "fmt"
import "net/http"
import "strings"
import "io/ioutil"

type ServiceCheck struct {
	Type             string
	Url              string
	HttpHost         string
	Username         string
	Password         string
	HostPort         string
	DummyResult      bool
	ExpectHttpStatus string
	ExpectString     string
	ConnectTimeout   int
	ResponseTimeout  int
	Delay            int
}

type CheckResult struct {
	ServiceName  string
	Endpoint     string
	Ok           bool
	EndpointInfo *EndpointInfo
}

// Rules how to check if a service is up or not
type ServiceChecks struct {
	ServiceName  string
	EndpointPort int
	Checks       []ServiceCheck
	EndpointInfo *EndpointInfo
}

type CheckEngine struct {
	jobs            chan ServiceChecks
	results         chan CheckResult
	configurations  chan MachineConfiguration
	endpointAddress string
}

func (ce *CheckEngine) Start(workers int, configResultPublisher ConfigResultPublisher, endpointAddress string, intervalInMs int) {
	ce.results = make(chan CheckResult, 100)
	ce.configurations = make(chan MachineConfiguration, 1)
	ce.endpointAddress = endpointAddress

	go CheckConfigUpdateWorker(ce.configurations, ce.results, endpointAddress, 1000)
	go PublishCheckResultWorker(ce.results, configResultPublisher)

}

func (ce *CheckEngine) Stop() {
	close(ce.jobs)
	close(ce.results)
	close(ce.configurations)
}

func (ce *CheckEngine) PushNewConfiguration(configuration MachineConfiguration) {
	ce.configurations <- configuration
}

func CheckConfigUpdateWorker(configurations <-chan MachineConfiguration, results chan<- CheckResult, endpointAddress string, delay int) {

	serviceCheckWorkerChannels := make(map[string]chan ServiceChecks)

	var configuration MachineConfiguration
	for {
		//fmt.Printf("pretick on %s\n", endpointAddress)
		newConf, alive := <-configurations
		//fmt.Printf("tick. Alive: %d, endpoint: %s\n", alive, endpointAddress)
		//select {
		//case newConf, alive := <-configurations:
		if alive {
			fmt.Printf("Got new configuration: %+v\n", newConf)

			configuration = newConf

			for name, c := range serviceCheckWorkerChannels {
				_, found := configuration.Services[name]
				if !found {
					// Service has been removed, close the channel
					fmt.Printf("Removing check %s from active duty\n", name)
					close(c)
					delete(serviceCheckWorkerChannels, name)
				}
			}

			for name, boundService := range configuration.Services {
				_, found := serviceCheckWorkerChannels[name]
				if !found {
					// New service
					fmt.Printf("Creating CheckServiceWorker for service %s\n", name)
					serviceCheckWorkerChannels[name] = make(chan ServiceChecks)
					go CheckServiceWorker(serviceCheckWorkerChannels[name], results, endpointAddress, delay)
				}

				service := boundService.GetConfig()
				var cc ServiceChecks
				cc.ServiceName = name
				cc.EndpointPort = service.EndpointPort
				cc.Checks = service.Checks
				if service.Container != nil {
					cc.EndpointInfo = &EndpointInfo{
						Revision:             service.GetRevision(),
						ServiceConfiguration: service,
					}
				}

				serviceCheckWorkerChannels[name] <- cc
			}
		} else {
			fmt.Printf("Closing CheckConfigUpdateWorker (got %d services)\n", len(serviceCheckWorkerChannels))
			for name, c := range serviceCheckWorkerChannels {
				fmt.Printf("Closing channel %s because we're closing CheckConfigUpdateWorker for %s\n", name, endpointAddress)
				close(c)
			}
			return
		}
		//}
		//	time.Sleep(time.Millisecond * time.Duration(500))
	}

}

func GetEndpointForContainer(service ServiceConfiguration) string {
	return "the-endpoint"
}

func PublishCheckResultWorker(results chan CheckResult, configResultPublisher ConfigResultPublisher) {
	for result := range results {
		//fmt.Printf("PublishCheckResultWorker %s %s\n", result.ServiceName, result.Ok)
		configResultPublisher.PublishServiceState(result.ServiceName, result.Endpoint, result.Ok, result.EndpointInfo)
	}
}

func CheckServiceWorker(serviceChecksChannel <-chan ServiceChecks, results chan<- CheckResult, endpointAddress string, delay int) {

	var serviceChecks ServiceChecks

	alive := true
	for alive {
		select {
		case newServiceChecks, alive := <-serviceChecksChannel:
			serviceChecks = newServiceChecks

			if !alive {
				fmt.Printf("Stopping CheckServiceWorker for service %s\n", serviceChecks.ServiceName)
				return
			} else {
				fmt.Printf("New check configuration for service %s\n", serviceChecks.ServiceName)
			}

		default:
			//fmt.Printf("Checking if service %s is up\n", serviceChecks.ServiceName)
			var result CheckResult
			result.ServiceName = serviceChecks.ServiceName
			result.Endpoint = fmt.Sprintf("%s:%d", endpointAddress, serviceChecks.EndpointPort)
			result.EndpointInfo = serviceChecks.EndpointInfo
			result.Ok = true
			ok := true
			for _, check := range serviceChecks.Checks {
				if check.Delay > 0 {
					delay = check.Delay
				}
				switch check.Type {
				case "dummy":
					ok = CheckDummyService(check)
				case "http":
					ok = CheckHttpService(check)
				case "tcp":
					ok = CheckTcpService(check)
				}
				if !ok {
					result.Ok = false
				}
			}

			if ok {
				results <- result
			}

		}

		time.Sleep(time.Millisecond * time.Duration(delay))
	}

}

type TimeoutConfig struct {
	ConnectTimeout   time.Duration
	ReadWriteTimeout time.Duration
}

func TimeoutDialer(config *TimeoutConfig) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, config.ConnectTimeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(config.ReadWriteTimeout))
		return conn, nil
	}
}

func CheckHttpService(check ServiceCheck) (ok bool) {
	ok = true

	config := &TimeoutConfig{
		ConnectTimeout:   300 * time.Millisecond,
		ReadWriteTimeout: 300 * time.Millisecond,
	}

	if check.ConnectTimeout > 0 {
		config.ConnectTimeout = time.Duration(check.ConnectTimeout) * time.Millisecond
	}

	if check.ResponseTimeout > 0 {
		config.ReadWriteTimeout = time.Duration(check.ResponseTimeout) * time.Millisecond
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: TimeoutDialer(config),
		},
	}

	req, _ := http.NewRequest("GET", check.Url, nil)
	if check.HttpHost != "" {
		req.Host = check.HttpHost
	}

	if check.Username != "" || check.Password != "" {
		req.SetBasicAuth(check.Username, check.Password)
	}

	//fmt.Printf("Checking http url %s\n", check.Url)
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return false
	}
	// fmt.Printf("resp: %+v, err: %+v\n\n", resp, err)

	if check.ExpectHttpStatus != "" && !strings.HasPrefix(resp.Status, check.ExpectHttpStatus) {
		//fmt.Printf("ExpectHttpStatus %s but status was %s\n", check.ExpectHttpStatus, resp.Status)
		ok = false
	}

	if check.ExpectHttpStatus == "" && !strings.HasPrefix(resp.Status, "200") {
		//fmt.Printf("status was not 200 but %s", resp.Status)
		ok = false
	}

	if check.ExpectString != "" {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			//fmt.Printf("ExpectString %s but error on ioutil.ReadAll: %+v\n", check.ExpectString, err)
			ok = false
		}

		if strings.Index(string(body), check.ExpectString) == -1 {
			//fmt.Printf("ExpectString %s but did not find. body: %s\n", check.ExpectString, body)
			ok = false
		}
	}

	return ok
}

func CheckDummyService(check ServiceCheck) (ok bool) {
	return check.DummyResult
}

func CheckTcpService(check ServiceCheck) bool {

	timeout := time.Millisecond * 50

	if check.ConnectTimeout > 0 {
		timeout = time.Duration(check.ConnectTimeout) * time.Millisecond
	}

	var deadline = time.Now().Add(timeout)
	conn, err := net.DialTimeout("tcp", check.HostPort, timeout)
	if conn != nil {
		conn.SetDeadline(deadline)
		defer conn.Close()
	}
	if err != nil {
		return false
	}

	return true
}
