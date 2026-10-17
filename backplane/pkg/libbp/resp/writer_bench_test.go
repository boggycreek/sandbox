// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package resp

import (
	"strconv"
	"testing"
)

func BenchmarkEncodeCommand(b *testing.B) {
	args := []string{"SET", "mykey", "myvalue", "EX", "60"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommand(args...)
	}
}

func BenchmarkEncodeCommandLarge(b *testing.B) {
	var args []string
	for i := 0; i < 50; i++ {
		args = append(args, "arg"+strconv.Itoa(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommand(args...)
	}
}

func BenchmarkEncodeCommandLargePayload(b *testing.B) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = 'a'
	}
	payloadStr := string(payload)
	args := []string{"SET", "mykey", payloadStr, "EX", "60"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommand(args...)
	}
}
