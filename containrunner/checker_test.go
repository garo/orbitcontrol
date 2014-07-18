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

func (s *CheckerSuite) TestCheckService(c *C) {

	checks := []ServiceCheck{{Type: "dummy", DummyResult: true}}

	ok := CheckService(checks)
	c.Assert(ok, Equals, true)
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

func (s *CheckerSuite) TestIndividualCheckWorker(c *C) {
	checkFalse := ServiceCheck{Type: "http", Url: "http://localhost:10/returnFoobar", ExpectString: "OK"}

	jobs := make(chan ServiceChecks, 10)
	results := make(chan CheckResult, 10)

	go IndividualCheckWorker(1, jobs, results)
	jobs <- ServiceChecks{"failingService", []ServiceCheck{checkFalse}}
	close(jobs)

	result := <-results

	c.Assert(result.Ok, Equals, false)
	c.Assert(result.ServiceName, Equals, "failingService")
}

type TestConfigResultPublisher struct {
}

func (c TestConfigResultPublisher) PublishServiceState(serviceName string, endpoint string, result bool) {
	if serviceName != "okService" || result != true || endpoint != "da-endpoint" {
		panic("TestPublishCheckResultWorker test failed")
	}
}

func (s *CheckerSuite) TestPublishCheckResultWorker(c *C) {

	results := make(chan CheckResult)

	var rp TestConfigResultPublisher

	go PublishCheckResultWorker(results, rp)
	results <- CheckResult{"okService", "da-endpoint", true}
	close(results)

}

func (s *CheckerSuite) TestCheckIntervalWorker(c *C) {

	configurations := make(chan MachineConfiguration, 1)
	jobsChannel := make(chan ServiceChecks, 100)

	var mc MachineConfiguration
	mc.Services = make(map[string]ServiceConfiguration)
	v := ServiceConfiguration{}
	v.Name = "myContainer"
	v.Checks = []ServiceCheck{{"dummyCheck", "", true, "", ""}}
	mc.Services["myContainer"] = v

	go CheckIntervalWorker(configurations, jobsChannel)
	configurations <- mc
	time.Sleep(time.Millisecond * 150)
	close(configurations)
	cc := <-jobsChannel

	c.Assert(cc.ServiceName, Equals, "myContainer")
	c.Assert(cc.Checks[0].Type, Equals, "dummyCheck")

}
