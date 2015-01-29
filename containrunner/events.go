package containrunner

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"os"
	"reflect"
	"time"
)

type MessageQueuer interface {
	Init(amqp_address string) error
	Declare() error
	GetReceiveredEventChannel() <-chan OrbitEvent
	PublishOrbitEvent(event OrbitEvent) error
}

type RabbitMQQueuer struct {
	amqp_address          string
	conn                  *amqp.Connection
	ch                    *amqp.Channel
	disconnected          chan *amqp.Error
	deploymentEventsQueue *amqp.Queue
	receiveredEvents      chan OrbitEvent
}

const rabbitmqRetryInterval = 5 * time.Second

type OrbitEvent struct {
	Ts    time.Time
	Type  string
	Event *json.RawMessage
	Ptr   interface{} `json:"-"`
}

func (e *OrbitEvent) String() string {
	bytes, _ := json.Marshal(e.Ptr)
	rawMsg := json.RawMessage(bytes)
	e.Event = &rawMsg

	bytes, _ = json.Marshal(e)
	return string(bytes)
}

type RelaunchContainerEvent struct {
	Name string
}

type DeploymentEvent struct {
	Service  string
	User     string
	Revision string
}

type NoopEvent struct {
	Data string
}

func NewOrbitEvent(event interface{}) OrbitEvent {
	oe := OrbitEvent{}
	oe.Ts = time.Now()
	oe.Type = reflect.TypeOf(event).Name()
	oe.Ptr = event

	return oe
}

func NewOrbitEventFromString(str string) (OrbitEvent, error) {
	var e OrbitEvent
	err := json.Unmarshal([]byte(str), &e)
	if err != nil {
		return e, err
	}

	switch e.Type {
	case "NoopEvent":
		var ee NoopEvent
		err := json.Unmarshal(*e.Event, &ee)
		if err != nil {
			return e, err
		}
		e.Ptr = ee
		break
	case "DeploymentEvent":
		var ee DeploymentEvent
		err := json.Unmarshal(*e.Event, &ee)
		if err != nil {
			return e, err
		}
		e.Ptr = ee
		break
	case "RelaunchContainerEvent":
		var ee RelaunchContainerEvent
		err := json.Unmarshal(*e.Event, &ee)
		if err != nil {
			return e, err
		}
		e.Ptr = ee
		break

	}

	return e, nil
}

func (d *RabbitMQQueuer) Init(amqp_address string) error {

	if d.receiveredEvents == nil {
		d.receiveredEvents = make(chan OrbitEvent)
	}

	d.amqp_address = amqp_address

	var disconnected chan *amqp.Error
	connected := make(chan bool)
	connected2 := make(chan bool)

	go d.Connect(connected)

	go func() {
		for {
			select {
			case <-connected:
				fmt.Printf("Connected to rabbitmq broker...\n")
				// Enable disconnect channel
				d.Declare()
				connected2 <- true
				disconnected = d.Disconnected()
			case errd := <-disconnected:
				// Disable disconnect channel
				disconnected = nil

				fmt.Printf("RabbitMQ disconnected: %s", errd)

				time.Sleep(10 * time.Second)
				go d.Connect(connected)
			}
		}
	}()

	<-connected2
	return nil

}

func (d *RabbitMQQueuer) Connect(connected chan bool) {
	reset := make(chan bool)
	done := make(chan bool)
	timer := time.AfterFunc(0, func() {
		d.connect(d.amqp_address, done)
		reset <- true
	})
	defer timer.Stop()

	for {
		select {
		case <-done:
			fmt.Println("RabbitMQ connected and channel established")
			connected <- true
			return
		case <-reset:
			timer.Reset(rabbitmqRetryInterval)
		}
	}
}

func (d *RabbitMQQueuer) Disconnected() chan *amqp.Error {
	return d.disconnected
}

func (d *RabbitMQQueuer) connect(uri string, done chan bool) {
	var err error

	fmt.Printf("dialing %q", uri)
	d.conn, err = amqp.Dial(uri)
	if err != nil {
		fmt.Printf("Dial: %s", err)
		return
	}

	fmt.Printf("Connection established, getting Channel")
	d.ch, err = d.conn.Channel()
	if err != nil {
		fmt.Printf("Channel: %s", err)
		return
	}

	// Notify disconnect channel when disconnected
	d.disconnected = make(chan *amqp.Error)
	d.ch.NotifyClose(d.disconnected)

	done <- true
}

func (d *RabbitMQQueuer) Declare() error {

	//
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
	//

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

	q, err := d.ch.QueueDeclare(
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
	d.deploymentEventsQueue = &q

	err = d.ch.QueueBind(
		q.Name, // queue name
		"#",    // routing key
		"orbitctl.deployment_events", // exchange
		false,
		nil)

	msgs, err := d.ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto ack
		false,  // exclusive
		false,  // no local
		false,  // no wait
		nil,    // args
	)
	fmt.Printf("Starting to consume on queue %s\n", q.Name)
	go d.eventConsumer(d.receiveredEvents, msgs)

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
		fmt.Fprintf(os.Stderr, "Got error, reconnecting to rabbitmq broker. err: %+v", err)
		d.Init(d.amqp_address)
	}

	return err
}
