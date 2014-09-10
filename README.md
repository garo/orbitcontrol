orbitcontrol
============

Orbit Control is a toolset to run containers in distributed set of machines, dynamically configure haproxies and run health checks for services.

Features
========
 - Launch Docker Containers into machines defined by tags
 - Supports health checks for services across a set of machines defined by tags
 - Configures haproxies so that they can route requests to these services
 - Full support of ephemeral machines (think of autoscaling arrays in the cloud)
 - Uses etcd to store run-time configuration.
 - Can easily use Git or other SCM to store the configuration so that history can be fully stored, also forms as a DR solution if the etcd cluster is lost.
 - Written in Go, only single binary needs to be deployed to machines.

Introduction
============

You have set of <strong>services</strong> which each have an unique service name. Each service should have an <strong>health check</strong>, which tells orbit if the service is up or down. Optionally each service can be defined to exists inside a Docker <strong>container</strong>. Orbit takes care that the correct container with correct revision is started.

Then you have set of machines which are divided into <strong>tags</strong>. Each tag can be told which services should exists in the machine specified by the tag. In other words: <strong>services are bound to tags</strong>. For example you have a tag "webserver-array" which has service "nginx" and tag "database" which has service "mysql".

Each tag can also have a <strong>haproxy</strong> configuration which can contain (in haproxy terminology) <em>frontends and backends</em>. The services can be bound to the haproxy instances with creating haproxy configuration files in a simple template language. For example you can have a tag "haproxy-frontends" which contains haproxy configuration which directs all requests to all available "nginx" services.

Orbit takes care of checking the status of each service, updating haproxy configurations and verifying that the correct containers are running for each containerised service.

Most state is stored in a simple directory structure containing json files. This structure should be stored in a revision control so that a clear version history can be maintained. After making changes a simple "orbitctl import" command is run to update the orbit configuration to the <strong>etcd</strong> cluster. This allows for easy disaster recovery in case the etcd cluster is destroyed.

For services deployed in containers a separated <strong>revision</strong> setting is stored only in etcd. If a service is developed in-house with frequent updates then this service revision can be assigned directly with the <em>orbitctl</em> cli command, bypassing the static orbit control .json files inside the version control. This allows the developers to deploy new versions without having to edit files, commit to the vcs and then issuing the <em>orbitctl import</em> command.

Dependencies
============

Orbit control is created with the Go language. This makes deployment really easy because only a single statically linked binary needs to be distributed to each machine. Only external dependency is the etcd configuration service.

In addition you mostly likely want to use haproxy and docker. 

Installation
============

These are rough installation instructions. Notice that orbitctl is not yet production quality.

1) Setup etcd cluster or use an existing. Orbitctl will place its data under /orbit directory by default.

2) Setup Git or other SCM where you store your orbit configuration. Example configuration can be found under the testdata/ directory.

3) Build the orbictl command with "make" command.

4) After editing the configurations use the <em>orbitctl import</em> command to import the configuration into etcd. Usually when you want to edit the configuration you first make changes to the files, commit them into Git and then run the orbitctl import.

5) Deploy the orbitctl binary to your machines. You can use Chef, Puppet or any other tool you are comfortable with.

6) Start the orbitctl on the machines with these arguments (modify to suit your taste): "--etcd-endpoint=[server1],[server2] daemon --machine-address=[machine internal ip] --machine-tags=tag1,tag2". There's an example upstart script under upstart directory.

That's it. Orbitctls should now be running on your machines and they should start the containers you have specified and also configure the haproxies to each machine which you have specified in the configuration.
