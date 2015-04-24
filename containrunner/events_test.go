package containrunner

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {

	e := NewOrbitEvent(NoopEvent{"test"})

	assert.Equal(t, "NoopEvent", e.Type, "NoopEvent")
	assert.Equal(t, "test", e.Ptr.(NoopEvent).Data)
}

func TestEventToStr(t *testing.T) {

	e := NewOrbitEvent(NoopEvent{"test"})
	str := e.String()

	assert.Equal(t, "{\"Ts\":\""+e.Ts.Format(time.RFC3339Nano)+"\",\"Type\":\"NoopEvent\",\"Event\":{\"Data\":\"test\"}}", str)
}

func TestNewOrbitEventFromString(t *testing.T) {

	str := "{\"Ts\":\"2015-01-28T08:29:56.381443454Z\",\"Type\":\"NoopEvent\",\"Event\":{\"Data\":\"test\"}}"

	e, err := NewOrbitEventFromString(str)
	assert.Nil(t, err)
	assert.Equal(t, e.Ts.Format(time.RFC3339Nano), "2015-01-28T08:29:56.381443454Z")
	assert.Equal(t, e.Type, "NoopEvent")
	assert.Equal(t, e.Ptr.(NoopEvent).Data, "test")

}
