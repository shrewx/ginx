package ginx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParameterBindingWithInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type formRouter struct {
		PathID      int    `in:"path" name:"id"`
		QueryName   string `in:"query" name:"name"`
		HeaderToken string `in:"header" name:"X-Token"`
		FormValue   string `in:"form" name:"formVal"`
		URLEncoded  string `in:"urlencoded" name:"encodedVal"`
		CookieValue string `in:"cookies" name:"session"`
	}

	router := &formRouter{}
	typeInfo := parseOperatorType(reflect.TypeOf(router))

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	body := strings.NewReader("formVal=foo&encodedVal=bar")
	req := httptest.NewRequest(http.MethodPost, "/test?name=neo", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Token", "token123")
	req.AddCookie(&http.Cookie{Name: "session", Value: "cookie-value"})
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: "42"}}

	InjectParsedParams(ctx)

	err := ParameterBinding(ctx, router, typeInfo)
	require.NoError(t, err)

	require.Equal(t, 42, router.PathID)
	require.Equal(t, "neo", router.QueryName)
	require.Equal(t, "token123", router.HeaderToken)
	require.Equal(t, "foo", router.FormValue)
	require.Equal(t, "bar", router.URLEncoded)
	require.Equal(t, "cookie-value", router.CookieValue)

	params := GetParsedParams(ctx)
	require.NotNil(t, params)

	pathParams := params["path"].(map[string]interface{})
	queryParams := params["query"].(map[string]interface{})
	headerParams := params["header"].(map[string]interface{})
	formParams := params["form"].(map[string]interface{})
	cookieParams := params["cookies"].(map[string]interface{})

	require.Equal(t, 42, pathParams["id"])
	require.Equal(t, "neo", queryParams["name"])
	require.Equal(t, "token123", headerParams["X-Token"])
	require.Equal(t, "foo", formParams["formVal"])
	require.Equal(t, "bar", formParams["encodedVal"])
	require.Equal(t, "cookie-value", cookieParams["session"])
}

func TestParameterBindingBodyInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type bodyPayload struct {
		Data string `json:"data"`
	}
	type bodyRouter struct {
		Body bodyPayload `in:"body" json:"body"`
	}

	typeInfo := parseOperatorType(reflect.TypeOf(&bodyRouter{}))

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"data":"payload"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	router := &bodyRouter{}
	InjectParsedParams(ctx)

	err := ParameterBinding(ctx, router, typeInfo)
	require.NoError(t, err)
	require.Equal(t, "payload", router.Body.Data)

	params := GetParsedParams(ctx)
	require.NotNil(t, params)
	bodyParams := params["body"].(map[string]interface{})
	require.Equal(t, "payload", bodyParams["data"])
}

func TestBindBodyParamContentTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type bodyVariants struct {
		Value string `json:"value" xml:"value" yaml:"value" toml:"value"`
	}

	fieldMeta := reflect.TypeOf(struct {
		Body bodyVariants `in:"body"`
	}{}).Field(0)

	type testCase struct {
		name        string
		contentType string
		payload     string
	}

	cases := []testCase{
		{"json", "application/json", `{"value":"json"}`},
		{"xml", "application/xml", `<bodyVariants><value>xml</value></bodyVariants>`},
		{"yaml", "application/x-yaml", "value: yaml"},
		{"toml", "application/toml", "value = \"toml\""},
		{"default", "", `{"value":"default"}`},
	}

	expected := map[string]string{
		"json":    "json",
		"xml":     "xml",
		"yaml":    "yaml",
		"toml":    "toml",
		"default": "default",
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.payload))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			ctx.Request = req

			wrapper := &struct {
				Body bodyVariants `in:"body"`
			}{}

			fieldVal := reflect.ValueOf(wrapper).Elem().Field(0)
			fieldInfo := FieldInfo{StructField: fieldMeta}

			err := bindBodyParam(ctx, fieldVal, fieldInfo)
			require.NoError(t, err)
			require.Equal(t, expected[tt.name], wrapper.Body.Value)
		})
	}
}

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

	result := structToMap(reflect.ValueOf(val), false)
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

	flattenResult := structToMap(reflect.ValueOf(flat), true)
	require.Equal(t, "flat", flattenResult["value"])
	require.Equal(t, "keep", flattenResult["plain"])
}

func TestConvertSliceAndMapToMap(t *testing.T) {
	type nested struct {
		Value string `json:"value"`
	}

	slice := []nested{{Value: "one"}}
	convertedSlice := convertSliceToMap(reflect.ValueOf(slice), true).([]interface{})
	require.Equal(t, "one", convertedSlice[0].(map[string]interface{})["value"])

	rawSlice := []int{1, 2}
	require.Equal(t, rawSlice, convertSliceToMap(reflect.ValueOf(rawSlice), false))

	m := map[string]nested{"foo": {Value: "bar"}}
	convertedMap := convertMapToMap(reflect.ValueOf(m), true).(map[string]interface{})
	require.Equal(t, "bar", convertedMap["foo"].(map[string]interface{})["value"])

	rawMap := map[string]int{"a": 1}
	require.Equal(t, rawMap, convertMapToMap(reflect.ValueOf(rawMap), false))
}

func TestGetFieldNamePriority(t *testing.T) {
	type sample struct {
		NameTag   string `name:"custom" json:"ignored"`
		JSONTag   string `json:"jsonName,omitempty"`
		Default   string
		OmitEmpty string `json:"-"`
	}

	typ := reflect.TypeOf(sample{})
	require.Equal(t, "custom", getFieldName(typ.Field(0)))
	require.Equal(t, "jsonName", getFieldName(typ.Field(1)))
	require.Equal(t, "default", getFieldName(typ.Field(2)))
	require.Equal(t, "omitEmpty", getFieldName(typ.Field(3)))
}

func TestInjectAndGetParsedParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	require.Nil(t, GetParsedParams(ctx))

	InjectParsedParams(ctx)
	ctx.Set(ParsedParamsKey, map[string]interface{}{"query": map[string]string{"name": "neo"}})

	params := GetParsedParams(ctx)
	require.Equal(t, "neo", params["query"].(map[string]string)["name"])
}
