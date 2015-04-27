package containrunner

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

type Webserver struct {
	lastKeepalive time.Time
	haproxyOk     time.Time
	server        *http.Server
	listener      *net.Listener
	Containrunner *Containrunner
	lock          sync.Mutex
}

type StatusData struct {
	services map[string]ServiceConfiguration
}

func (ce *Webserver) Keepalive() {
	ce.lock.Lock()
	defer ce.lock.Unlock()
	ce.lastKeepalive = time.Now()
}

func (ce *Webserver) checkHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Service", "orbit")
	ce.lock.Lock()
	lastKeepalive := ce.lastKeepalive
	ce.lock.Unlock()

	if time.Since(lastKeepalive) < time.Minute {
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

func (ce *Webserver) Start(port int) error {
	ce.server = new(http.Server)
	ce.server.Addr = fmt.Sprintf(":%d", port)
	mux := http.NewServeMux()
	ce.server.Handler = mux

	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	mux.HandleFunc("/check", ce.checkHandler)
	mux.HandleFunc("/status", ce.statusHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/html")
		asset, _ := Asset("src/github.com/garo/orbitcontrol/data/index.html")
		w.Write(asset)
	})

	mux.HandleFunc("/dashboard.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/css")
		asset, _ := Asset("src/github.com/garo/orbitcontrol/data/dashboard.css")
		w.Write(asset)
	})

	listener, err := net.Listen("tcp", ce.server.Addr)
	if err != nil {
		return err
	}
	ce.listener = &listener

	go func() {
		ce.server.Serve(*ce.listener)
	}()

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
