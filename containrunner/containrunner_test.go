package containrunner

import . "gopkg.in/check.v1"

//import "fmt"
//import "strings"

type ContainrunnerSuite struct {
}

var _ = Suite(&ContainrunnerSuite{})

func (s *ContainrunnerSuite) SetUpTest(c *C) {

}

func (s *ContainrunnerSuite) TestEventHandlerWithNoopEvent(c *C) {

	var incomingLoopbackEvents chan OrbitEvent = make(chan OrbitEvent)
	go EventHandler(nil, incomingLoopbackEvents)

	incomingLoopbackEvents <- NewOrbitEvent(NoopEvent{"test"})

}

/*
func (s *ContainrunnerSuite) TestStart(c *C) {
	var cs Containrunner
	cs.Tags = []string{"mytag"}
	cs.Start()

}

*/
