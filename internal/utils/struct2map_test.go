package utils

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructToMapHelpers(t *testing.T) {
	type nested struct {
		Value string `json:"value"`
	}
	type sample struct {
		Number    int               `json:"number"`
		Nested    nested            `json:"nested"`
		NestedPtr *nested           `json:"nestedPtr"`
		Slice     []nested          `json:"slice"`
		Map       map[string]nested `json:"map"`
		Plain     string
	}

	val := &sample{
		Number:    7,
		Nested:    nested{Value: "nested"},
		NestedPtr: &nested{Value: "ptr"},
		Slice:     []nested{{Value: "s1"}, {Value: "s2"}},
		Map:       map[string]nested{"k": {Value: "mv"}},
		Plain:     "plain",
	}

	result := StructToMap(reflect.ValueOf(val), false)
	require.Equal(t, 7, result["number"])
	require.Equal(t, "nested", result["nested"].(map[string]interface{})["value"])
	require.Equal(t, "ptr", result["nestedPtr"].(map[string]interface{})["value"])

	sliceResult := result["slice"].([]interface{})
	require.Len(t, sliceResult, 2)
	require.Equal(t, "s1", sliceResult[0].(map[string]interface{})["value"])

	mapResult := result["map"].(map[string]interface{})
	require.Equal(t, "mv", mapResult["k"].(map[string]interface{})["value"])
	require.Equal(t, "plain", result["plain"])

	type flattenSample struct {
		Inner nested `json:"inner"`
		Plain string `json:"plain"`
	}

	flat := flattenSample{
		Inner: nested{Value: "flat"},
		Plain: "keep",
	}

	flattenResult := StructToMap(reflect.ValueOf(flat), true)
	require.Equal(t, "flat", flattenResult["value"])
	require.Equal(t, "keep", flattenResult["plain"])
}

func TestConvertSliceAndMapToMap(t *testing.T) {
	type nested struct {
		Value string `json:"value"`
	}

	slice := []nested{{Value: "one"}}
	convertedSlice := ConvertSliceToMap(reflect.ValueOf(slice), true).([]interface{})
	require.Equal(t, "one", convertedSlice[0].(map[string]interface{})["value"])

	rawSlice := []int{1, 2}
	require.Equal(t, rawSlice, ConvertSliceToMap(reflect.ValueOf(rawSlice), false))

	m := map[string]nested{"foo": {Value: "bar"}}
	convertedMap := ConvertMapToMap(reflect.ValueOf(m), true).(map[string]interface{})
	require.Equal(t, "bar", convertedMap["foo"].(map[string]interface{})["value"])

	rawMap := map[string]int{"a": 1}
	require.Equal(t, rawMap, ConvertMapToMap(reflect.ValueOf(rawMap), false))
}

func TestGetFieldNamePriority(t *testing.T) {
	type sample struct {
		NameTag   string `name:"custom" json:"ignored"`
		JSONTag   string `json:"jsonName,omitempty"`
		Default   string
		OmitEmpty string `json:"-"`
	}

	typ := reflect.TypeOf(sample{})
	require.Equal(t, "custom", GetFieldName(typ.Field(0)))
	require.Equal(t, "jsonName", GetFieldName(typ.Field(1)))
	require.Equal(t, "default", GetFieldName(typ.Field(2)))
	require.Equal(t, "omitEmpty", GetFieldName(typ.Field(3)))
}

