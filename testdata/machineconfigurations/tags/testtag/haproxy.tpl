global
stats socket /var/run/haproxy/admin.sock level admin user haproxy group haproxy
	log 127.0.0.1	local2 info
	maxconn 16000
 	ulimit-n 40000
	user haproxy
	group haproxy
	daemon
	quiet
	pidfile /var/run/haproxy.pid

defaults
	log	global
	mode http
	option httplog
	option dontlognull
	retries 3
	option redispatch
	maxconn	8000
	contimeout 5000
	clitimeout 60000
	srvtimeout 60000

