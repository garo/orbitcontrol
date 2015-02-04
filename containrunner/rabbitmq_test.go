package containrunner

import (
	"fmt"
	. "gopkg.in/check.v1"
	"time"
)

type RabbitMQSuite struct {
	queuer RabbitMQQueuer
}

var _ = Suite(&RabbitMQSuite{})

func (s *RabbitMQSuite) SetUpTest(c *C) {
	fmt.Printf("Connecting to broker\n")

	err := s.queuer.Init("amqp://guest:guest@localhost:5672/", "")
	c.Assert(err, Equals, nil)
	fmt.Printf("Broker: %+v\n", s.queuer.deploymentEventsQueue)

}

func (s *RabbitMQSuite) TestPublishAndConsume(c *C) {
	e := NewOrbitEvent(DeploymentEvent{
		"action",
		"service name",
		"user name",
		"revision id",
		"machine address",
	})

	fmt.Printf("Publishing to mq\n")

	err := s.queuer.PublishOrbitEvent(e)
	c.Assert(err, Equals, nil)

	receiver := s.queuer.GetReceiveredEventChannel()
	event := <-receiver

	c.Assert(event.Type, Equals, "DeploymentEvent")
}
