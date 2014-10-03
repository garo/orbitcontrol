package containrunner

import "testing"
import "fmt"
import "time"

type TestLoggingEvent struct {
	Foobar int
}

func BenchmarkJSONLogging(b *testing.B) {
	for i := 0; i < b.N; i++ {
		LogEvent(TestLoggingEvent{i})
	}
}

func BenchmarkTraditionalLogging(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("TestLoggingEvent: %s, %d", time.Now().Format(time.RFC3339), i)
	}
}
