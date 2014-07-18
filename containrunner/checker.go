package containrunner

import "net"
import "time"

//import "fmt"
import "net/http"
import "strings"
import "io/ioutil"

type CheckResult struct {
	ServiceName string
	Endpoint    string
	Ok          bool
}

type ServiceChecks struct {
	ServiceName string
	Checks      []ServiceCheck
}

type CheckEngine struct {
	jobs           chan ServiceChecks
	results        chan CheckResult
	configurations chan MachineConfiguration
}

func (ce *CheckEngine) Start(workers int, configResultPublisher ConfigResultPublisher) {
	ce.jobs = make(chan ServiceChecks, 100)
	ce.results = make(chan CheckResult, 100)
	ce.configurations = make(chan MachineConfiguration, 1)

	for w := 1; w <= workers; w++ {
		go IndividualCheckWorker(w, ce.jobs, ce.results)
	}

	go CheckIntervalWorker(ce.configurations, ce.jobs)
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

func CheckIntervalWorker(configurations <-chan MachineConfiguration, jobsChannel chan<- ServiceChecks) {
	var configuration *MachineConfiguration
	alive := true
	for alive {
		select {
		case newConf, alive := <-configurations:
			if alive {
				configuration = &newConf
			}
		default:
			if configuration != nil {
				for name, service := range configuration.Services {
					var cc ServiceChecks
					cc.ServiceName = name
					cc.Checks = service.Checks
					//fmt.Printf("Pushing check %+v\n", cc)
					jobsChannel <- cc
				}
			}
		}
		time.Sleep(time.Millisecond * 100)

	}

}

func GetEndpointForContainer(service ServiceConfiguration) string {
	return "the-endpoint"
}

func PublishCheckResultWorker(results chan CheckResult, configResultPublisher ConfigResultPublisher) {
	for result := range results {
		configResultPublisher.PublishServiceState(result.ServiceName, result.Endpoint, result.Ok)
	}
}

func IndividualCheckWorker(id int, jobs <-chan ServiceChecks, results chan<- CheckResult) {
	for j := range jobs {
		var result CheckResult
		result.ServiceName = j.ServiceName
		result.Ok = CheckService(j.Checks)
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
		}

		if !ok {
			return false
		}
	}

	return ok
}

func CheckHttpService(check ServiceCheck) (ok bool) {
	ok = true

	transport := http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, time.Duration(100*time.Millisecond))
		},
	}

	client := http.Client{Transport: &transport}

	//fmt.Printf("Checking http url %s\n", check.Url)
	resp, err := client.Get(check.Url)
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
