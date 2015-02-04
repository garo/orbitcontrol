package containrunner

import (
	"errors"
	"fmt"
	"github.com/streadway/amqp"
	"os"
	"time"
)

// State machine definition for rabbitmq connection
type AMQPState int

const (
	AMQPNotStarted AMQPState = iota
	AMQPConnecting
	AMQPTryingAgain
	AMQPFinalizingConnection
	AMQPConnected
	AMQPDisconnected
)

func (s AMQPState) String() string {
	switch s {
	case AMQPNotStarted:
		return "AMQPNotStarted"
	case AMQPConnecting:
		return "AMQPConnecting"
	case AMQPTryingAgain:
		return "AMQPTryingAgain"
	case AMQPFinalizingConnection:
		return "AMQPFinalizingConnection"
	case AMQPConnected:
		return "AMQPConnected"
	case AMQPDisconnected:
		return "AMQPDisconnected"
	default:
		return "<AMQP Unknown state>"
	}
}

type RabbitMQQueuer struct {
	amqp_address          string
	conn                  *amqp.Connection
	ch                    *amqp.Channel
	disconnected          chan *amqp.Error
	deploymentEventsQueue *amqp.Queue
	receiveredEvents      chan OrbitEvent
	listen_queue_name     string
	State                 AMQPState
}

const rabbitmqRetryInterval = 5 * time.Second

// Tries to connect to AMQP.
// Returns true if connection was established within a few seconds, otherwise false.
// If connections was not established it will be tried in the background continuously
func (d *RabbitMQQueuer) Init(amqp_address string, listen_queue_name string) bool {

	if d.receiveredEvents == nil {
		d.receiveredEvents = make(chan OrbitEvent)
	}

	d.amqp_address = amqp_address
	d.listen_queue_name = listen_queue_name

	connected := make(chan bool)

	d.State = AMQPNotStarted

	go func() {
		for {
			done := make(chan bool)

			// connect creates d.disconnected channel which will receiver an error if the AMQP connection closes
			for {
				d.State = AMQPConnecting
				err := d.connect(d.amqp_address, done)
				if err != nil {
					d.State = AMQPTryingAgain
					fmt.Printf("Connection refused. Trying again in 5 seconds...\n")
					time.Sleep(5 * time.Second)
				} else {
					d.State = AMQPFinalizingConnection
					fmt.Printf("Connected to rabbitmq broker...\n")
					// Enable disconnect channel
					err := d.Declare()
					if err != nil {
						fmt.Printf("Error on Declare: %+v\n", err)
						continue
					}
					err = d.ListenDeploymentEventsExchange(listen_queue_name, d.receiveredEvents)
					if err != nil {
						fmt.Printf("Error on ListenDeploymentEventsExchange: %+v\n", err)
						continue
					}

					d.State = AMQPConnected
					// This doesn't block if the channel is already full
					select {
					case connected <- true:
					default:
					}

					break
				}
			}

			fmt.Printf("Connection loop will now wait for a disconnect...\n")
			err := <-d.disconnected
			d.State = AMQPDisconnected
			fmt.Printf("AMQP got disconnected. trying again in a second. error: %+v\n", err)
			d.ch = nil
			time.Sleep(1 * time.Second)
		}
	}()

	select {
	case <-connected:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

func (d *RabbitMQQueuer) connect(uri string, done chan bool) error {
	var err error

	fmt.Printf("dialing %q\n", uri)
	d.conn, err = amqp.Dial(uri)
	if err != nil {
		fmt.Printf("Dial: %s\n", err)
		return err
	}

	fmt.Printf("Connection established, getting Channel\n")
	d.ch, err = d.conn.Channel()
	if err != nil {
		fmt.Printf("Channel: %s", err)
		return err
	}

	// Notify disconnect channel when disconnected
	d.disconnected = make(chan *amqp.Error)
	d.ch.NotifyClose(d.disconnected)

	return nil
}

// Declare all orbitctl queues and exchanges
//
// deployment_events flow:
//
// 1) All messages are published to orbitctl.deployment_events exchange
//
// 2) Persistent queue orbitctl.deployment_events_persistent_storage is bound to this exchange
//    and its used to store all deployment events until they are written to a permanent database storage.
//
// 3) Other listeners can listen for deployment events by defining temporary anonymous queues
func (d *RabbitMQQueuer) Declare() error {

	err := d.ch.ExchangeDeclare(
		"orbitctl.deployment_events", // name
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to declare orbitctl.deployment_events topic exchange: %+v", err)
		return err
	}

	_, err = d.ch.QueueDeclare(
		"orbitctl.deployment_events_persistent_storage", // name
		true,  // durable
		false, // delete when usused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to declare orbitctl.deployment_events_persistent_storage queue: %+v", err)
		return err
	}

	err = d.ch.QueueBind(
		"orbitctl.deployment_events_persistent_storage", // queue name
		"#", // routing key
		"orbitctl.deployment_events", // exchange
		false,
		nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind orbitctl.deployment_events_persistent_storage queue to orbitctl.deployment_events exchange: %+v", err)
		return err
	}

	return nil
}

// Starts a listening goroutine on for events coming to orbitctl.deployment_events exchange.
// The queue is passed in queue_name variable or if its empty an anonymous queue is created.
func (d *RabbitMQQueuer) ListenDeploymentEventsExchange(queue_name string, receivered_events chan OrbitEvent) error {

	if queue_name == "" {

		queue, err := d.ch.QueueDeclare(
			"",    // name
			false, // durable
			false, // delete when usused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to declare return queue: %+v", err)
			return err
		}

		err = d.ch.QueueBind(
			queue.Name, // queue name
			"#",        // routing key
			"orbitctl.deployment_events", // exchange
			false,
			nil)

		queue_name = queue.Name
	}

	msgs, err := d.ch.Consume(
		queue_name, // queue
		"",         // consumer
		true,       // auto ack
		false,      // exclusive
		false,      // no local
		false,      // no wait
		nil,        // args
	)
	if err != nil {
		return err
	}

	fmt.Printf("Starting to consume on queue %s\n", queue_name)
	go d.eventConsumer(receivered_events, msgs)

	return nil
}

func (d *RabbitMQQueuer) eventConsumer(destination chan<- OrbitEvent, msgs <-chan amqp.Delivery) {
	for delivery := range msgs {
		orbitEvent, err := NewOrbitEventFromString(string(delivery.Body))
		if err != nil {
			fmt.Printf("Got error on json unmarshal: %+v. Discarding message.\n", err)
		} else {
			destination <- orbitEvent
		}

	}
}

func (d *RabbitMQQueuer) GetReceiveredEventChannel() <-chan OrbitEvent {
	return d.receiveredEvents
}

func (d *RabbitMQQueuer) Close() {
	d.ch.Close()
	d.conn.Close()
	d.ch = nil
	d.conn = nil
}

func (d *RabbitMQQueuer) PublishOrbitEvent(oe OrbitEvent) error {
	if d.State != AMQPConnected {
		return errors.New(fmt.Sprintf("AMQP not connected (was %s). Dropping message %+v\n", d.State, oe))
	}

	err := d.ch.Publish(
		"orbitctl.deployment_events", // exchange
		oe.Type, // routing key
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         []byte(oe.String()),
			DeliveryMode: 2,
		})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Got error on send: %+v", err)
	}

	return err
}
