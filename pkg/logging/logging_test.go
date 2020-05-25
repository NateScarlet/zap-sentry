package logging

import "testing"

func BenchmarkHubLogger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hub := Hub{}
		hub.Logger("test")
	}
}
