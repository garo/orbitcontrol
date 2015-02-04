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

func (s *EventsSuite) TestNewEvent(c *C) {

	e := NewOrbitEvent(NoopEvent{"test"})

	c.Assert(e.Type, Equals, "NoopEvent")
	c.Assert(e.Ptr.(NoopEvent).Data, Equals, "test")
}

func (s *EventsSuite) TestEventToStr(c *C) {

	e := NewOrbitEvent(NoopEvent{"test"})
	str := e.String()

	c.Assert(str, Equals, "{\"Ts\":\""+e.Ts.Format(time.RFC3339Nano)+"\",\"Type\":\"NoopEvent\",\"Event\":{\"Data\":\"test\"}}")
}

func (s *EventsSuite) TestNewOrbitEventFromString(c *C) {

	str := "{\"Ts\":\"2015-01-28T08:29:56.381443454Z\",\"Type\":\"NoopEvent\",\"Event\":{\"Data\":\"test\"}}"

	e, err := NewOrbitEventFromString(str)
	c.Assert(err, Equals, nil)
	c.Assert(e.Ts.Format(time.RFC3339Nano), Equals, "2015-01-28T08:29:56.381443454Z")
	c.Assert(e.Type, Equals, "NoopEvent")
	c.Assert(e.Ptr.(NoopEvent).Data, Equals, "test")

}
