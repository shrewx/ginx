package enum

import (
	"errors"
	"fmt"
)

var InvalidTypeError = errors.New("invalid enum type error")

type Enum interface {
	Int() int
	String() string
	Label() string
	Type() string
	Values() []Enum
}

type Offset interface {
	Offset() int
}

type Value struct {
	Key         string
	StringValue *string
	IntValue    *int64
	FloatValue  *float64
	Label       string
}

func (e Value) Type() interface{} {
	if e.IntValue != nil {
		return *e.IntValue
	}
	if e.FloatValue != nil {
		return *e.FloatValue
	}
	if e.StringValue != nil {
		return *e.StringValue
	}

	return nil
}

type Values []Value

func (o Values) Len() int {
	return len(o)
}

func (o Values) Types() []interface{} {
	types := make([]interface{}, len(o))

	for i, v := range o {
		types[i] = v.Type()
	}

	return types
}

func (o Values) Less(i, j int) bool {
	if o[i].FloatValue != nil {
		return *o[i].FloatValue < *o[j].FloatValue
	}
	if o[i].IntValue != nil {
		return *o[i].IntValue < *o[j].IntValue
	}
	return *o[i].StringValue < *o[j].StringValue
}

func (o Values) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func ToInt64(i interface{}) (int64, error) {
	switch v := i.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case uint:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", i)
	}
}
