package containrunner

import "time"
import "reflect"
import "encoding/json"

type LogEvent struct {
	Timestamp string      `json:"ts"`
	T         string      `json:"t"`
	E         interface{} `json:"e"`
}

func LogString(e interface{}) string {
	le := LogEvent{time.Now().Format(time.RFC3339), reflect.TypeOf(e).Name(), e}
	bytearr, err := json.Marshal(le)
	if err != nil {
		panic(err)
	}
	return string(bytearr)
}
