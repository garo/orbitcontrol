package containrunner

import (
	"io/ioutil"
	"net/http"
	"testing"
)

func TestCheckHandler(t *testing.T) {

	server := new(Webserver)
	err := server.Start(64123)
	if err != nil {
		t.Fail()
	}
	resp, err := http.Get("http://localhost:64123/check")
	if err != nil {
		t.Fail()
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) == "OK\n" {
		t.Fail()
	}

	server.Keepalive()

	resp, err = http.Get("http://localhost:64123/check")
	if err != nil {
		t.Fail()
	}
	body, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != "OK\n" {
		t.Fail()
	}

	server.Close()

}
