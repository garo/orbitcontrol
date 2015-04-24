package containrunner

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDummyService(t *testing.T) {
	checkTrue := ServiceCheck{Type: "dummy", DummyResult: true}
	checkFalse := ServiceCheck{Type: "dummy", DummyResult: false}

	assert.Equal(t, true, CheckDummyService(checkTrue))
	assert.Equal(t, false, CheckDummyService(checkFalse))
}

func TestCheckServiceWorker(t *testing.T) {
	serviceChecksChannel := make(chan ServiceChecks)
	results := make(chan OrbitEvent, 10)

	serviceChecks := ServiceChecks{}
	serviceChecks.Checks = []ServiceCheck{{Type: "dummy", DummyResult: true}}

	go CheckServiceWorker(serviceChecksChannel, results, "10.0.0.1", 10)
	serviceChecksChannel <- serviceChecks
	result := (<-results).Ptr.(ServiceStateEvent)

	assert.Equal(t, true, result.IsUp)
}

func TestTCPService(t *testing.T) {
	checkTrue := ServiceCheck{Type: "tcp", HostPort: "127.0.0.1:22"}
	checkFalse := ServiceCheck{Type: "tcp", HostPort: "127.0.0.1:1"}
	//checkTrueExpect := ServiceCheck{Type: "tcp", HostPort: "127.0.0.1:22", ExpectString: "SSH"}

	assert.Equal(t, true, CheckTcpService(checkTrue))
	assert.Equal(t, false, CheckTcpService(checkFalse))
}

func TestHttpService(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/check" {
			fmt.Fprintln(w, "OK")
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	checkTrue := ServiceCheck{Type: "http", Url: ts.URL + "/check"}
	checkFalse := ServiceCheck{Type: "http", Url: ts.URL + "/notFound"}

	assert.Equal(t, true, CheckHttpService(checkTrue))
	assert.Equal(t, false, CheckHttpService(checkFalse))

}

func TestHttpServiceExpectHttpStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/check" {
			fmt.Fprintln(w, "OK")
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	checkTrue := ServiceCheck{Type: "http", Url: ts.URL + "/check", ExpectHttpStatus: "200"}
	checkFalse := ServiceCheck{Type: "http", Url: ts.URL + "/notFound", ExpectHttpStatus: "404"}

	assert.Equal(t, true, CheckHttpService(checkTrue))
	assert.Equal(t, true, CheckHttpService(checkFalse))

}

func TestHttpServiceExpectHttpString(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/returnOK" {
			fmt.Fprintln(w, "OK")
		} else {
			fmt.Fprintln(w, "Foobar\n")
		}
	}))
	defer ts.Close()

	checkTrue := ServiceCheck{Type: "http", Url: ts.URL + "/returnOK", ExpectString: "OK"}
	checkFalse := ServiceCheck{Type: "http", Url: ts.URL + "/returnFoobar", ExpectString: "OK"}

	assert.Equal(t, true, CheckHttpService(checkTrue))
	assert.Equal(t, false, CheckHttpService(checkFalse))
}

func TestHttpServiceNotResponding(t *testing.T) {
	checkFalse := ServiceCheck{Type: "http", Url: "http://localhost:10/returnFoobar", ExpectString: "OK"}

	assert.Equal(t, false, CheckHttpService(checkFalse))
}

type TestConfigResultPublisher struct {
}

func (c TestConfigResultPublisher) PublishServiceState(serviceName string, endpoint string, result bool, info *EndpointInfo) {
	if serviceName != "okService" || result != true || endpoint != "da-endpoint" {
		panic("TestPublishCheckResultWorker test failed")
	}
}

func TestCheckConfigUpdateWorker(t *testing.T) {

	configurations := make(chan MachineConfiguration)
	resultsChannel := make(chan OrbitEvent, 1)

	var mc MachineConfiguration
	mc.Services = make(map[string]BoundService)
	v := ServiceConfiguration{}
	v.Name = "myService"
	v.Checks = []ServiceCheck{{
		Type:             "dummyCheck",
		Url:              "",
		HttpHost:         "",
		Username:         "",
		Password:         "",
		HostPort:         "",
		DummyResult:      true,
		ExpectHttpStatus: "",
		ExpectString:     ""}}
	boundService := BoundService{}
	boundService.DefaultConfiguration = v
	mc.Services["myService"] = boundService

	go CheckConfigUpdateWorker(configurations, resultsChannel, "10.0.0.1", 10)
	configurations <- mc
	result := (<-resultsChannel).Ptr.(ServiceStateEvent)
	close(configurations)

	assert.Equal(t, "myService", result.Service)
	assert.Equal(t, true, result.IsUp)
}

func TestCheckConfigUpdateWorkerWhenServiceIsRemoved(t *testing.T) {

	configurations := make(chan MachineConfiguration, 1)
	resultsChannel := make(chan OrbitEvent, 1)

	var mc MachineConfiguration
	mc.Services = make(map[string]BoundService)
	v := ServiceConfiguration{}
	v.Name = "myService"
	v.Checks = []ServiceCheck{{
		Type:             "dummyCheck",
		Url:              "",
		HttpHost:         "",
		Username:         "",
		Password:         "",
		HostPort:         "",
		DummyResult:      true,
		ExpectHttpStatus: "",
		ExpectString:     ""}}
	boundService := BoundService{}
	boundService.DefaultConfiguration = v
	mc.Services["myService"] = boundService

	go CheckConfigUpdateWorker(configurations, resultsChannel, "TestCheckConfigUpdateWorkerWhenServiceIsRemoved", 100)
	configurations <- mc
	time.Sleep(time.Millisecond * 150)
	fmt.Println("Removing service...")

	mc = MachineConfiguration{}
	mc.Services = make(map[string]BoundService)
	configurations <- mc
	time.Sleep(time.Millisecond * 200)
	fmt.Println("Closing CheckConfigUpdateWorker from the test")
	close(configurations)
	result := (<-resultsChannel).Ptr.(ServiceStateEvent)

	assert.Equal(t, "myService", result.Service)
	assert.Equal(t, true, result.IsUp)

}
