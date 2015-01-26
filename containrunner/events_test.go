package containrunner

import (
	"fmt"
	. "gopkg.in/check.v1"
	"time"
)

type EventsSuite struct {
	queuer RabbitMQQueuer
}

var _ = Suite(&EventsSuite{})

func (s *EventsSuite) SetUpTest(c *C) {
	fmt.Printf("Connecting to broker\n")

	err := s.queuer.Init("amqp://guest:guest@localhost:5672/")
	c.Assert(err, Equals, nil)
	fmt.Printf("Broker: %+v\n", s.queuer.deploymentEventsQueue)

}

func (s *EventsSuite) TestPublishAndConsume(c *C) {
	e := DeploymentEvent{
		"service name",
		"user name",
		"revision id",
	}

	fmt.Printf("Publishing to mq\n")

	t, _ := time.Parse(time.RFC3339, "2012-11-01T22:08:41+00:00")

	err := s.queuer.PublishDeploymentEvent(t, e)
	c.Assert(err, Equals, nil)

	receiver := s.queuer.GetReceiveredEventChannel()
	event := <-receiver

	c.Assert(event.Type, Equals, "DeploymentEvent")
}
