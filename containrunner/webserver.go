package containrunner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type Webserver struct {
	lastKeepalive time.Time
	haproxyOk     time.Time
	server        *http.Server
	listener      *net.Listener
	Containrunner *Containrunner
}

type StatusData struct {
	services map[string]ServiceConfiguration
}

func (ce *Webserver) Keepalive() {
	ce.lastKeepalive = time.Now()
}

func (ce *Webserver) checkHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Service", "orbit")
	if time.Since(ce.lastKeepalive) < time.Minute {
		fmt.Fprintf(w, "OK\n")
	} else {
		http.Error(w, "Keepalive timeout", 500)
	}
}

func (ce *Webserver) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Service", "orbit")
	w.Header().Set("Content-type", "text/javascript")
	services, err := ce.Containrunner.GetAllServices(nil)
	if err != nil {
		http.Error(w, "GetAllServices error: "+err.Error(), 500)
	}

	bytes, err := json.Marshal(services)
	if err != nil {
		http.Error(w, "json.Marshall error: "+err.Error(), 500)
	}

	w.Write(bytes)

}

func (ce *Webserver) Start(useEmbeddedResources bool) error {

	ce.server = new(http.Server)
	ce.server.Addr = ":1500"
	mux := http.NewServeMux()
	ce.server.Handler = mux

	if useEmbeddedResources {
		fmt.Printf("Using embedded resources\n")
	} else {
		fmt.Printf("Using external resources from src/github.com/garo/orbitcontrol/data/\n")
	}

	mux.HandleFunc("/check", ce.checkHandler)
	mux.HandleFunc("/status", ce.statusHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/html")
		if useEmbeddedResources {
			asset, _ := Asset("src/github.com/garo/orbitcontrol/data/index.html")
			w.Write(asset)
		} else {
			bytes, _ := ioutil.ReadFile("src/github.com/garo/orbitcontrol/data/index.html")
			fmt.Printf("Serving index.html:%s\n", string(bytes))
			w.Write(bytes)
		}
	})

	mux.HandleFunc("/dashboard.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/css")
		if useEmbeddedResources {
			asset, _ := Asset("src/github.com/garo/orbitcontrol/data/dashboard.css")
			w.Write(asset)
		} else {
			bytes, _ := ioutil.ReadFile("src/github.com/garo/orbitcontrol/data/dashboard.css")
			fmt.Printf("Serving dashboard.css:%s\n", string(bytes))
			w.Write(bytes)
		}
	})

	listener, err := net.Listen("tcp", ce.server.Addr)
	if err != nil {
		return err
	}
	ce.listener = &listener

	go func() {
		ce.server.Serve(*ce.listener)
	}()
	time.Sleep(9223372036854775807)
	return nil
}

func (ce *Webserver) Close() {
	// FIXME: Don't think the server close actually works.
	if ce.listener != nil {
		(*ce.listener).Close()
		ce.listener = nil
		ce.server = nil
	}
}
