package ginx

import (
	"bytes"
	"github.com/bytedance/sonic"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"golang.org/x/net/context"
	"net/http"
	"sort"
)

// HTTPRequestOption 用于自定义 HTTP 请求的选项函数类型
type HTTPRequestOption func(*http.Request)

func NewRequestWithContext(ctx context.Context, method string, rawUrl string, data interface{}, opts ...HTTPRequestOption) (*http.Request, error) {
	header := http.Header{}
	// 从上下文获取语言设置，支持国际化
	lang, ok := ctx.Value(CurrentLangHeader()).(string)
	if ok {
		header.Add(CurrentLangHeader(), lang)
	} else {
		header.Add(CurrentLangHeader(), ginx.i18nLang)
	}

	// 初始化各种参数容器
	body := new(bytes.Buffer) // 请求体
	if data != nil {
		marshalData, err := sonic.Marshal(data)
		if err != nil {
			return nil, err
		}
		body.Write(marshalData)
	}
	header.Set("Content-Type", MineApplicationJson)

	// 构建最终的HTTP请求
	req, err := http.NewRequestWithContext(ctx, method, rawUrl, body)
	if err != nil {
		return nil, err
	}
	
	req.Header = header

	// 应用 HTTPRequestOption 配置
	for _, opt := range opts {
		opt(req)
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	return req, nil
}

func WithHTTPHeaders(headers map[string]string) HTTPRequestOption {
	return func(req *http.Request) {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
}

func WithHTTPCookies(cookies ...*http.Cookie) HTTPRequestOption {
	return func(req *http.Request) {
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
	}
}

// WithHTTPAuthorization 添加 Authorization Header
func WithHTTPAuthorization(token string) HTTPRequestOption {
	return WithHTTPHeaders(map[string]string{"Authorization": token})
}

// WithHTTPContentType 设置 Content-Type
func WithHTTPContentType(contentType string) HTTPRequestOption {
	return WithHTTPHeaders(map[string]string{"Content-Type": contentType})
}

func CopyAllHeaders(ctxReq *http.Request) HTTPRequestOption {
	return func(req *http.Request) {
		for k, v := range ctxReq.Header {
			req.Header[k] = v
		}
	}
}

func CopyAllCookies(ctxReq *http.Request) HTTPRequestOption {
	return func(req *http.Request) {
		cookies := ctxReq.Cookies()
		// 添加Cookie（按名称排序以确保一致性）
		if n := len(cookies); n > 0 {
			// 创建 cookie 的副本并按名称排序
			cookieList := make([]*http.Cookie, n)
			copy(cookieList, cookies)
			sort.Slice(cookieList, func(i, j int) bool {
				return cookieList[i].Name < cookieList[j].Name
			})

			for _, cookie := range cookieList {
				req.AddCookie(cookie)
			}
		}
	}
}
