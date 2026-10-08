// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package resp

import (
	"fmt"
	"io"
	"strconv"
)

// Writer writes formatted RESP commands to an output stream
type Writer struct {
	w io.Writer
}

// NewWriter creates a new RESP writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteCommand serializes a command and its string arguments as a RESP Array of Bulk Strings
func (w *Writer) WriteCommand(args ...string) error {
	if len(args) == 0 {
		return nil
	}
	header := fmt.Sprintf("*%d\r\n", len(args))
	if _, err := io.WriteString(w.w, header); err != nil {
		return err
	}
	for _, arg := range args {
		chunk := fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
		if _, err := io.WriteString(w.w, chunk); err != nil {
			return err
		}
	}
	return nil
}

// WriteValue serializes any RESP Value struct to the wire
func (w *Writer) WriteValue(v Value) error {
	if v.IsNull {
		_, err := io.WriteString(w.w, "$-1\r\n")
		return err
	}

	switch v.Type {
	case TypeSimpleString:
		_, err := fmt.Fprintf(w.w, "+%s\r\n", v.Str)
		return err

	case TypeError:
		_, err := fmt.Fprintf(w.w, "-%s\r\n", v.Str)
		return err

	case TypeInteger:
		_, err := fmt.Fprintf(w.w, ":%d\r\n", v.Num)
		return err

	case TypeBulkString:
		_, err := fmt.Fprintf(w.w, "$%d\r\n%s\r\n", len(v.Str), v.Str)
		return err

	case TypeArray, TypePush, TypeSet:
		if _, err := fmt.Fprintf(w.w, "*%d\r\n", len(v.Array)); err != nil {
			return err
		}
		for _, item := range v.Array {
			if err := w.WriteValue(item); err != nil {
				return err
			}
		}
		return nil

	case TypeBoolean:
		if v.Boolean {
			_, err := io.WriteString(w.w, "#t\r\n")
			return err
		}
		_, err := io.WriteString(w.w, "#f\r\n")
		return err

	case TypeNull:
		_, err := io.WriteString(w.w, "_\r\n")
		return err

	default:
		// Default to bulk string encoding
		str := v.String()
		_, err := fmt.Fprintf(w.w, "$%d\r\n%s\r\n", len(str), str)
		return err
	}
}

// EncodeCommand produces the byte slice of a RESP array command
func EncodeCommand(args ...string) []byte {
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
