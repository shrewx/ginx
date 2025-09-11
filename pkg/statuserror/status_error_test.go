package statuserror

import (
	"fmt"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/shrewx/ginx/internal/fields"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/stretchr/testify/assert"
)

// Test StatusErr Localize with mocked generated fields, WithParams and WithField
func TestStatusErr_Localize_WithParams_And_WithField(t *testing.T) {
	// prepare i18n
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// mock like generated fields for a custom code
	const mockCode int64 = 40000009999
	add := func(lang, id, other string) { i18nx.AddMessages(lang, []*i18n.Message{{ID: id, Other: other}}) }

	// error_codes.<code> with template {{.Name}}
	add("zh", fmt.Sprintf("zh.error_codes.%d", mockCode), "{{.Name}} 不存在")
	add("en", fmt.Sprintf("en.error_codes.%d", mockCode), "{{.Name}} not exist")
	// references: line label
	add("zh", "zh.errors.references.line", "行")
	add("en", "en.errors.references.line", "line")
	// a normal field label used via string key
	add("zh", "zh.fields.detail", "详情")
	add("en", "en.fields.detail", "detail")

	tests := []struct {
		name   string
		lang   string
		expect string
	}{
		{
			name:   "zh formatting",
			lang:   "zh",
			expect: "行:1\nUser 不存在\n>> 详情:x",
		},
		{
			name:   "en formatting",
			lang:   "en",
			expect: "line:1\nUser not exist\n>> detail:x",
		},
	}

	for _, tt := range tests {
		// build error and fill params/fields
		err := NewStatusErr("NotFoundByName", mockCode)
		err.WithParams(map[string]interface{}{"Name": "User"}).
			WithField(i18nx.NewMessage("line", "errors.references"), "1").
			WithField("fields.detail", "x")

		// localize
		msg := err.Localize(i18nx.Instance(), tt.lang)
		assert.Equal(t, tt.expect, msg.Value(), tt.name)
	}
}

// Test StatusErr Localize with ErrList (multiple errors)
func TestStatusErr_Localize_WithErrList(t *testing.T) {
	// prepare i18n
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// mock fields for multiple error codes
	const code1, code2 int64 = 40000009998, 40000009997
	add := func(lang, id, other string) { i18nx.AddMessages(lang, []*i18n.Message{{ID: id, Other: other}}) }

	add("zh", fmt.Sprintf("zh.error_codes.%d", code1), "用户 {{.Name}} 不存在")
	add("en", fmt.Sprintf("en.error_codes.%d", code1), "User {{.Name}} not found")
	add("zh", fmt.Sprintf("zh.error_codes.%d", code2), "权限不足")
	add("en", fmt.Sprintf("en.error_codes.%d", code2), "Insufficient permissions")

	tests := []struct {
		name   string
		lang   string
		expect string
	}{
		{
			name:   "zh multiple errors",
			lang:   "zh",
			expect: "用户 Alice 不存在\n权限不足",
		},
		{
			name:   "en multiple errors",
			lang:   "en",
			expect: "User Alice not found\nInsufficient permissions",
		},
	}

	for _, tt := range tests {
		// build error with ErrList
		err := NewStatusErr("MultipleErrors", 40000009999)
		err.ErrList = []map[string]interface{}{
			{
				"statusErr": &StatusErr{
					K:         "UserNotFound",
					ErrorCode: code1,
					Params:    map[string]interface{}{"Name": "Alice"},
				},
			},
			{
				"statusErr": &StatusErr{
					K:         "InsufficientPermissions",
					ErrorCode: code2,
				},
			},
		}

		// localize
		msg := err.Localize(i18nx.Instance(), tt.lang)
		assert.Equal(t, tt.expect, msg.Value(), tt.name)
	}
}

// Test StatusErr Localize with ErrList showing index information
func TestStatusErr_Localize_WithErrList_WithIndex(t *testing.T) {
	// prepare i18n
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// mock fields for multiple error codes with index information
	const code1, code2, code3 int64 = 40000009996, 40000009995, 40000009994
	add := func(lang, id, other string) { i18nx.AddMessages(lang, []*i18n.Message{{ID: id, Other: other}}) }

	// Error messages
	add("zh", fmt.Sprintf("zh.error_codes.%d", code1), "用户名 {{.Name}} 格式不正确")
	add("en", fmt.Sprintf("en.error_codes.%d", code1), "Username {{.Name}} format is invalid")
	add("zh", fmt.Sprintf("zh.error_codes.%d", code2), "密码强度不足")
	add("en", fmt.Sprintf("en.error_codes.%d", code2), "Password strength is insufficient")
	add("zh", fmt.Sprintf("zh.error_codes.%d", code3), "邮箱 {{.Email}} 已被使用")
	add("en", fmt.Sprintf("en.error_codes.%d", code3), "Email {{.Email}} is already in use")

	// Index field labels
	add("zh", "zh.errors.references.err_index", "索引")
	add("en", "en.errors.references.err_index", "index")

	tests := []struct {
		name   string
		lang   string
		expect string
	}{
		{
			name:   "zh multiple errors with index",
			lang:   "zh",
			expect: "索引:1\n用户名 test@invalid 格式不正确\n>> 索引:1\n索引:2\n密码强度不足\n>> 索引:2\n索引:3\n邮箱 user@example.com 已被使用\n>> 索引:3",
		},
		{
			name:   "en multiple errors with index",
			lang:   "en",
			expect: "index:1\nUsername test@invalid format is invalid\n>> index:1\nindex:2\nPassword strength is insufficient\n>> index:2\nindex:3\nEmail user@example.com is already in use\n>> index:3",
		},
	}

	for _, tt := range tests {
		// build error with ErrList containing index information
		err := NewStatusErr("ValidationErrors", 40000009999)
		err.ErrList = []map[string]interface{}{
			{
				"statusErr": &StatusErr{
					K:         "InvalidUsername",
					ErrorCode: code1,
					Params:    map[string]interface{}{"Name": "test@invalid"},
					Fields:    map[interface{}]string{fields.ErrorIndex: "1"},
				},
			},
			{
				"statusErr": &StatusErr{
					K:         "WeakPassword",
					ErrorCode: code2,
					Fields:    map[interface{}]string{fields.ErrorIndex: "2"},
				},
			},
			{
				"statusErr": &StatusErr{
					K:         "EmailAlreadyExists",
					ErrorCode: code3,
					Params:    map[string]interface{}{"Email": "user@example.com"},
					Fields:    map[interface{}]string{fields.ErrorIndex: "3"},
				},
			},
		}

		// localize
		msg := err.Localize(i18nx.Instance(), tt.lang)
		assert.Equal(t, tt.expect, msg.Value(), tt.name)
	}
}

// Test StatusErr Summary method
func TestStatusErr_Summary(t *testing.T) {
	err := NewStatusErr("TestError", 40000000001)
	summary := err.Summary()
	expected := "[TestError][40000000001]"
	assert.Equal(t, expected, summary)
}

// Test StatusErr StatusCode method
func TestStatusErr_StatusCode(t *testing.T) {
	tests := []struct {
		name     string
		code     int64
		expected int
	}{
		{"400 error", 40000000001, 400},
		{"500 error", 50000000001, 500},
		{"200 error", 20000000001, 200},
		{"short code", 42, 0}, // less than 3 digits
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStatusErr("TestError", tt.code)
			statusCode := err.StatusCode()
			assert.Equal(t, tt.expected, statusCode)
		})
	}
}

// Test StatusErr Error method
func TestStatusErr_Error(t *testing.T) {
	err := NewStatusErr("TestError", 40000000001)
	errorMsg := err.Error()
	expected := "[TestError][40000000001]"
	assert.Equal(t, expected, errorMsg)
}

// Test StatusErr Key method
func TestStatusErr_Key(t *testing.T) {
	err := NewStatusErr("TestError", 40000000001)
	key := err.Key()
	assert.Equal(t, "TestError", key)
}

// Test StatusCodeFromCode function
func TestStatusCodeFromCode(t *testing.T) {
	tests := []struct {
		name     string
		code     int64
		expected int
	}{
		{"400 error", 40000000001, 400},
		{"500 error", 50000000001, 500},
		{"200 error", 20000000001, 200},
		{"short code", 42, 0}, // less than 3 digits
		{"single digit", 5, 0},
		{"two digits", 99, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StatusCodeFromCode(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}
