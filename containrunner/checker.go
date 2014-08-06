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
	HostPort         string
	DummyResult      bool
	ExpectHttpStatus string
	ExpectString     string
}

type CheckResult struct {
	ServiceName  string
	Endpoint     string
	Ok           bool
	EndpointInfo *EndpointInfo
}

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
	ce.jobs = make(chan ServiceChecks, 100)
	ce.results = make(chan CheckResult, 100)
	ce.configurations = make(chan MachineConfiguration, 1)
	ce.endpointAddress = endpointAddress

	for w := 1; w <= workers; w++ {
		go IndividualCheckWorker(w, ce.jobs, ce.results, endpointAddress)
	}

	go CheckIntervalWorker(ce.configurations, ce.jobs, intervalInMs)
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

func CheckIntervalWorker(configurations <-chan MachineConfiguration, jobsChannel chan<- ServiceChecks, intervalInMs int) {
	var configuration *MachineConfiguration
	alive := true
	for alive {
		select {
		case newConf, alive := <-configurations:
			if alive {
				fmt.Printf("Got new configuration: %+v\n", newConf)

				configuration = &newConf
			}
		default:
			if configuration != nil {
				//fmt.Printf("services: %+v\n", configuration.Services)

				for name, service := range configuration.Services {
					var cc ServiceChecks
					cc.ServiceName = name
					cc.EndpointPort = service.EndpointPort
					cc.Checks = service.Checks
					if service.Container != nil {
						cc.EndpointInfo = &EndpointInfo{
							Revision: service.GetRevision(),
						}
					}
					//fmt.Printf("Pushing check %+v\n", cc)
					jobsChannel <- cc
				}
			}
		}
		time.Sleep(time.Millisecond * time.Duration(intervalInMs))

	}

}

func GetEndpointForContainer(service ServiceConfiguration) string {
	return "the-endpoint"
}

func PublishCheckResultWorker(results chan CheckResult, configResultPublisher ConfigResultPublisher) {
	for result := range results {
		configResultPublisher.PublishServiceState(result.ServiceName, result.Endpoint, result.Ok, result.EndpointInfo)
	}
}

func IndividualCheckWorker(id int, jobs <-chan ServiceChecks, results chan<- CheckResult, endpointAddress string) {
	for j := range jobs {
		var result CheckResult
		result.ServiceName = j.ServiceName
		result.Endpoint = fmt.Sprintf("%s:%d", endpointAddress, j.EndpointPort)
		result.Ok = CheckService(j.Checks)
		result.EndpointInfo = j.EndpointInfo
		results <- result
	}
}

func CheckService(checks []ServiceCheck) (ok bool) {

	for _, check := range checks {
		switch check.Type {
		case "dummy":
			ok = CheckDummyService(check)
		case "http":
			ok = CheckHttpService(check)
		case "tcp":
			ok = CheckTcpService(check)
		}

		if !ok {
			return false
		}
	}

	return ok
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
		ConnectTimeout:   100 * time.Millisecond,
		ReadWriteTimeout: 100 * time.Millisecond,
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: TimeoutDialer(config),
		},
	}

	//fmt.Printf("Checking http url %s\n", check.Url)
	resp, err := client.Get(check.Url)
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
