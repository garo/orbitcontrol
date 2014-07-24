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
