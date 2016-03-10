package containrunner

import (
	"testing"
)

func TestEventHandlerWithNoopEvent(t *testing.T) {

	cr := Containrunner{}
	cr.EtcdEndpoints = []string{"http://localhost:4001"}

	var incomingLoopbackEvents chan OrbitEvent = make(chan OrbitEvent)
	go cr.EventHandler(nil, incomingLoopbackEvents)

	incomingLoopbackEvents <- NewOrbitEvent(NoopEvent{"test"})

}
