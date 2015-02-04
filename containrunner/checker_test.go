package containrunner

//import "testing"

//import "fmt"
import . "gopkg.in/check.v1"
import "net/http"
import "net/http/httptest"
import "fmt"
import "time"

//import "github.com/stretchr/testify/mock"

type CheckerSuite struct {
}

var _ = Suite(&CheckerSuite{})

func (s *CheckerSuite) SetUpTest(c *C) {
}

func (s *CheckerSuite) TestDummyService(c *C) {
	checkTrue := ServiceCheck{Type: "dummy", DummyResult: true}
	checkFalse := ServiceCheck{Type: "dummy", DummyResult: false}

	c.Assert(CheckDummyService(checkTrue), Equals, true)
	c.Assert(CheckDummyService(checkFalse), Equals, false)
}

func (s *CheckerSuite) TestCheckServiceWorker(c *C) {
	serviceChecksChannel := make(chan ServiceChecks)
	results := make(chan OrbitEvent, 10)

	serviceChecks := ServiceChecks{}
	serviceChecks.Checks = []ServiceCheck{{Type: "dummy", DummyResult: true}}

	go CheckServiceWorker(serviceChecksChannel, results, "10.0.0.1", 10)
	serviceChecksChannel <- serviceChecks
	result := (<-results).Ptr.(ServiceStateEvent)

	c.Assert(result.IsUp, Equals, true)
}

func (s *CheckerSuite) TestTCPService(c *C) {
	checkTrue := ServiceCheck{Type: "tcp", HostPort: "127.0.0.1:22"}
	checkFalse := ServiceCheck{Type: "tcp", HostPort: "127.0.0.1:1"}
	//checkTrueExpect := ServiceCheck{Type: "tcp", HostPort: "127.0.0.1:22", ExpectString: "SSH"}

	c.Assert(CheckTcpService(checkTrue), Equals, true)
	c.Assert(CheckTcpService(checkFalse), Equals, false)
}

func (s *CheckerSuite) TestHttpService(c *C) {
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

	c.Assert(CheckHttpService(checkTrue), Equals, true)
	c.Assert(CheckHttpService(checkFalse), Equals, false)
}

func (s *CheckerSuite) TestHttpServiceExpectHttpStatus(c *C) {
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

	c.Assert(CheckHttpService(checkTrue), Equals, true)
	c.Assert(CheckHttpService(checkFalse), Equals, true)
}

func (s *CheckerSuite) TestHttpServiceExpectHttpString(c *C) {
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

	c.Assert(CheckHttpService(checkTrue), Equals, true)
	c.Assert(CheckHttpService(checkFalse), Equals, false)
}

func (s *CheckerSuite) TestHttpServiceNotResponding(c *C) {
	checkFalse := ServiceCheck{Type: "http", Url: "http://localhost:10/returnFoobar", ExpectString: "OK"}

	c.Assert(CheckHttpService(checkFalse), Equals, false)
}

type TestConfigResultPublisher struct {
}

func (c TestConfigResultPublisher) PublishServiceState(serviceName string, endpoint string, result bool, info *EndpointInfo) {
	if serviceName != "okService" || result != true || endpoint != "da-endpoint" {
		panic("TestPublishCheckResultWorker test failed")
	}
}

func (s *CheckerSuite) TestCheckConfigUpdateWorker(c *C) {

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

	c.Assert(result.Service, Equals, "myService")
	c.Assert(result.IsUp, Equals, true)
}

func (s *CheckerSuite) TestCheckConfigUpdateWorkerWhenServiceIsRemoved(c *C) {
	fmt.Println("********* TestCheckConfigUpdateWorkerWhenServiceIsRemoved start ")

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
	delete(mc.Services, "myService")
	configurations <- mc
	time.Sleep(time.Millisecond * 200)
	fmt.Println("Closing CheckConfigUpdateWorker from the test")
	close(configurations)
	result := (<-resultsChannel).Ptr.(ServiceStateEvent)

	c.Assert(result.Service, Equals, "myService")
	c.Assert(result.IsUp, Equals, true)
	fmt.Println("********* TestCheckConfigUpdateWorkerWhenServiceIsRemoved end ")

}
