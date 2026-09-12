package resp

import (
	"errors"
	"fmt"
	"strconv"
)

// Type prefixes in RESP protocol
const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
	TypeNull         = '_'
	TypeBoolean      = '#'
	TypeDouble       = ','
	TypeBigNumber    = '('
	TypeBulkError    = '!'
	TypeVerbatim     = '='
	TypeMap          = '%'
	TypeSet          = '~'
	TypePush         = '>'
)

var (
	// ErrNil indicates a null RESP response
	ErrNil = errors.New("nil response")
	// ErrProtocol indicates a RESP protocol parsing error
	ErrProtocol = errors.New("protocol error")
)

// Value represents a generic RESP value
type Value struct {
	Type    byte
	Str     string
	Num     int64
	Array   []Value
	Map     map[string]Value
	Boolean bool
	IsNull  bool
}

// String returns a human-readable representation of the RESP Value
func (v Value) String() string {
	switch v.Type {
	case TypeSimpleString, TypeError, TypeBulkString, TypeVerbatim:
		if v.IsNull {
			return "<nil>"
		}
		return v.Str
	case TypeInteger:
		return strconv.FormatInt(v.Num, 10)
	case TypeBoolean:
		return strconv.FormatBool(v.Boolean)
	case TypeArray:
		if v.IsNull {
			return "<nil array>"
		}
		return fmt.Sprintf("%v", v.Array)
	case TypeMap:
		return fmt.Sprintf("%v", v.Map)
	default:
		if v.IsNull {
			return "<nil>"
		}
		return v.Str
	}
}

// AsString returns the string representation or error if not string-like
func (v Value) AsString() (string, error) {
	if v.IsNull {
		return "", ErrNil
	}
	switch v.Type {
	case TypeSimpleString, TypeBulkString, TypeError, TypeVerbatim:
		return v.Str, nil
	case TypeInteger:
		return strconv.FormatInt(v.Num, 10), nil
	default:
		return "", fmt.Errorf("%w: cannot convert type %c to string", ErrProtocol, v.Type)
	}
}

// AsInt returns integer value or error
func (v Value) AsInt() (int64, error) {
	if v.IsNull {
		return 0, ErrNil
	}
	if v.Type == TypeInteger {
		return v.Num, nil
	}
	if v.Type == TypeBulkString || v.Type == TypeSimpleString {
		n, err := strconv.ParseInt(v.Str, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: invalid integer string %q", ErrProtocol, v.Str)
		}
		return n, nil
	}
	return 0, fmt.Errorf("%w: cannot convert type %c to integer", ErrProtocol, v.Type)
}

// AsArray returns the array of values or error
func (v Value) AsArray() ([]Value, error) {
	if v.IsNull {
		return nil, ErrNil
	}
	if v.Type != TypeArray && v.Type != TypePush && v.Type != TypeSet {
		return nil, fmt.Errorf("%w: value is not an array (type %c)", ErrProtocol, v.Type)
	}
	return v.Array, nil
}

// AsMap returns map representation or error
func (v Value) AsMap() (map[string]Value, error) {
	if v.IsNull {
		return nil, ErrNil
	}
	if v.Type == TypeMap {
		return v.Map, nil
	}
	// Also support parsing key-value arrays (common in RESP2)
	if v.Type == TypeArray {
		if len(v.Array)%2 != 0 {
			return nil, fmt.Errorf("%w: array length %d cannot be converted to map (odd count)", ErrProtocol, len(v.Array))
		}
		m := make(map[string]Value, len(v.Array)/2)
		for i := 0; i < len(v.Array); i += 2 {
			k := v.Array[i].String()
			m[k] = v.Array[i+1]
		}
		return m, nil
	}
	return nil, fmt.Errorf("%w: value is not a map or kv-array (type %c)", ErrProtocol, v.Type)
}
