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
