package resp

import (
	"testing"
)

func BenchmarkEncodeCommand(b *testing.B) {
	args := []string{"SET", "mykey", "myvalue", "EX", "60"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommand(args...)
	}
}
