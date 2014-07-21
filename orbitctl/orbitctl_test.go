package main

import (
	. "gopkg.in/check.v1"
	"testing"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type OrbitCtlSuite struct {
}

var _ = Suite(&OrbitCtlSuite{})

func (s *OrbitCtlSuite) TestGetEtcdEndpoints(c *C) {

	globalFlags.EtcdEndpoint = "http://etcd:4001"
	endpoints := GetEtcdEndpoints()
	c.Assert(endpoints[0], Equals, "http://etcd:4001")

	globalFlags.EtcdEndpoint = "http://etcd:4001,http://etcd-2:4001"
	endpoints = GetEtcdEndpoints()
	c.Assert(endpoints[0], Equals, "http://etcd:4001")
	c.Assert(endpoints[1], Equals, "http://etcd-2:4001")

}
