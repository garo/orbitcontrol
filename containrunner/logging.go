package containrunner

import "time"
import "reflect"
import "encoding/json"

type LogEventStruct struct {
	Timestamp string      `json:"ts"`
	T         string      `json:"t"`
	E         interface{} `json:"e"`
}

type LogMsg struct {
	Msg string
}

func LogString(msg string) string {
	return LogEvent(LogMsg{msg})
}

func LogEvent(e interface{}) string {
	le := LogEventStruct{time.Now().Format(time.RFC3339), reflect.TypeOf(e).Name(), e}
	bytearr, err := json.Marshal(le)
	if err != nil {
		panic(err)
	}
	return string(bytearr)
}
