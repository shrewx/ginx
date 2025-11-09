package statuserror

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteHTTPError_Methods(t *testing.T) {
	body := []byte("oops")
	headers := http.Header{}
	headers.Set("X-Test", "1")
	headers.Set("Content-Type", "application/problem+json")

	err := NewRemoteHTTPError(502, headers, body, "")

	assert.Equal(t, 502, err.Status())
	assert.Equal(t, body, err.Body())
	assert.Equal(t, headers, err.Headers())
	assert.Equal(t, "application/problem+json", err.ContentType())
	assert.Contains(t, err.Error(), "remote http 502")
	assert.Contains(t, err.Error(), "oops")
}

func TestRemoteHTTPError_ContentTypeFallback(t *testing.T) {
	// 没有 contentType，也没有 headers 中的 Content-Type，应回退为 application/json
	body := []byte("err")
	headers := http.Header{}

	err := NewRemoteHTTPError(400, headers, body, "")
	assert.Equal(t, "application/json", err.ContentType())

	// 显式传入 contentType 优先生效
	err2 := NewRemoteHTTPError(400, nil, body, "text/plain")
	assert.Equal(t, "text/plain", err2.ContentType())
}
