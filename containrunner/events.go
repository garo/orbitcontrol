package containrunner

import (
	"encoding/json"
	"reflect"
	"time"
)

type MessageQueuer interface {
	Init(amqp_address string, listen_queue_name string) bool
	Declare() error
	ListenDeploymentEventsExchange(queue_name string, receivered_events chan OrbitEvent) error
	GetReceiveredEventChannel() <-chan OrbitEvent
	PublishOrbitEvent(event OrbitEvent) error
}

type OrbitEvent struct {
	Ts    time.Time
	Type  string
	Event *json.RawMessage
	Ptr   interface{} `json:"-"`
}

type DeploymentEvent struct {
	Action         string
	Service        string
	User           string
	Revision       string
	MachineAddress string
	Jitter         int
}

type NoopEvent struct {
	Data string
}

type ServiceStateEvent struct {
	Service        string
	Endpoint       string
	IsUp           bool
	StateChanged   bool
	SameStateSince time.Time
	EndpointInfo   *EndpointInfo
}

func (e *OrbitEvent) String() string {
	bytes, _ := json.Marshal(e.Ptr)
	rawMsg := json.RawMessage(bytes)
	e.Event = &rawMsg

	bytes, _ = json.Marshal(e)
	return string(bytes)
}

func NewOrbitEvent(event interface{}) OrbitEvent {
	oe := OrbitEvent{}
	oe.Ts = time.Now()
	oe.Type = reflect.TypeOf(event).Name()
	oe.Ptr = event

	return oe
}

// Creates OrbitEvent from String. Used to Unmarshal messages from rabbitmq broker
//
// The sub type is stored in the OrbitEvent.Type field as string, which should be
// diretly the name of the event structure. This is used to create the appropriate stucture
// and its stored in the OrbitEvent.Ptr which accepts any interface{}.
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
	case "ServiceStateEvent":
		var ee ServiceStateEvent
		err := json.Unmarshal(*e.Event, &ee)
		if err != nil {
			return e, err
		}
		e.Ptr = ee
		break
	}

	return e, nil
}
