package ginx

const (
	Middleware = "middleware"
	APIKey     = "apiKey"
	BasicAuth  = "basicAuth"
	BearerJWT  = "bearerJWT"
)

type MiddlewareType struct{}

func (e *MiddlewareType) Type() string {
	return Middleware
}

type APIKeySecurityType struct{}

func (e *APIKeySecurityType) Type() string {
	return APIKey
}

type HTTPBasicAuthSecurityType struct{}

func (e *HTTPBasicAuthSecurityType) Type() string {
	return BasicAuth
}

type HTTPBearJWTSecurityType struct{}

func (e *HTTPBearJWTSecurityType) Type() string {
	return BearerJWT
}
