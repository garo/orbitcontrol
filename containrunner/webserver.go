package containrunner

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

type Webserver struct {
	lastKeepalive time.Time
	haproxyOk     time.Time
	server        *http.Server
	listener      *net.Listener
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

func (ce *Webserver) Start() error {

	ce.server = new(http.Server)
	ce.server.Addr = ":1500"
	mux := http.NewServeMux()
	ce.server.Handler = mux

	mux.HandleFunc("/check", ce.checkHandler)
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
