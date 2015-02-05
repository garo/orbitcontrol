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
	err := server.Start(64123)
	c.Assert(err, IsNil)
	resp, err := http.Get("http://localhost:64123/check")
	c.Assert(err, IsNil)
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	c.Assert(string(body), Not(Equals), "OK\n")

	server.Keepalive()

	resp, err = http.Get("http://localhost:64123/check")
	c.Assert(err, IsNil)
	body, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	c.Assert(string(body), Equals, "OK\n")

	server.Close()

}
