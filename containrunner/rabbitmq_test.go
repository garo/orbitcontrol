package containrunner

import (
	"fmt"
	"testing"
)

func TestPublishAndConsume(t *testing.T) {
	queuer := RabbitMQQueuer{}
	fmt.Printf("Connecting to broker\n")

	connected := queuer.Init("amqp://guest:guest@localhost:5672/", "")
	if connected != true {
		t.Fail()
	}
	fmt.Printf("Broker: %+v\n", queuer.deploymentEventsQueue)

	e := NewOrbitEvent(DeploymentEvent{
		"action",
		"service name",
		"user name",
		"revision id",
		"machine address",
		10,
	})

	fmt.Printf("Publishing to mq\n")

	err := queuer.PublishOrbitEvent(e)
	if err != nil {
		t.Fail()
	}

	receiver := queuer.GetReceiveredEventChannel()
	event := <-receiver

	if event.Type != "DeploymentEvent" {
		t.Fail()
	}
}
