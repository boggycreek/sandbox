package main

import (
	"strconv"
	"testing"
)

// The one the reviewer prefers
func EncodeCommandAppendInt(args ...string) []byte {
	var totalLen int
	totalLen += len(strconv.Itoa(len(args))) + 3 // "*<len>\r\n"
	for _, a := range args {
		totalLen += 1 + len(strconv.Itoa(len(a))) + 2 + len(a) + 2 // "$<len>\r\n<data>\r\n"
	}
	buf := make([]byte, 0, totalLen)
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)), 10)
	buf = append(buf, '\r', '\n')
	for _, a := range args {
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(a)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, a...)
		buf = append(buf, '\r', '\n')
	}
	return buf
}

func EncodeCommandOrig(args ...string) []byte {
	var totalLen int
	totalLen += len(strconv.Itoa(len(args))) + 3 // "*<len>\r\n"
	for _, a := range args {
		totalLen += 1 + len(strconv.Itoa(len(a))) + 2 + len(a) + 2 // "$<len>\r\n<data>\r\n"
	}
	buf := make([]byte, 0, totalLen)
	buf = append(buf, '*')
	buf = append(buf, strconv.Itoa(len(args))...)
	buf = append(buf, '\r', '\n')
	for _, a := range args {
		buf = append(buf, '$')
		buf = append(buf, strconv.Itoa(len(a))...)
		buf = append(buf, '\r', '\n')
		buf = append(buf, a...)
		buf = append(buf, '\r', '\n')
	}
	return buf
}

func BenchmarkOrigSmall(b *testing.B) {
	args := []string{"SET", "mykey", "myvalue", "EX", "60"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommandOrig(args...)
	}
}

func BenchmarkAppendIntSmall(b *testing.B) {
	args := []string{"SET", "mykey", "myvalue", "EX", "60"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommandAppendInt(args...)
	}
}

func BenchmarkOrigLargePayload(b *testing.B) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = 'a'
	}
	payloadStr := string(payload)
	args := []string{"SET", "mykey", payloadStr, "EX", "60"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommandOrig(args...)
	}
}

func BenchmarkAppendIntLargePayload(b *testing.B) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = 'a'
	}
	payloadStr := string(payload)
	args := []string{"SET", "mykey", payloadStr, "EX", "60"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeCommandAppendInt(args...)
	}
}
