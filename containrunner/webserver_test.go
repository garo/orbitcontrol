package containrunner

import (
	. "gopkg.in/check.v1"
	"io/ioutil"
	"net/http"
)

type WebserverSuite struct {
}

var _ = Suite(&WebserverSuite{})

func (s *WebserverSuite) SetUpTest(c *C) {

}

func (s *WebserverSuite) TestCheckHandler(c *C) {
	server := new(Webserver)
	server.Start()
	server.Keepalive()
	resp, err := http.Get("http://localhost:1500/check")
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	c.Assert(string(body), Equals, "OK\n")
	server.Close()
}

func (s *WebserverSuite) TestCheckHandlerNoKeepalive(c *C) {
	server := new(Webserver)
	server.Start()
	resp, err := http.Get("http://localhost:1500/check")
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	c.Assert(string(body), Equals, "OK\n")
	server.Close()
}
