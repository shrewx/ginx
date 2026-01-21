package ginx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/go-courier/reflectx"
	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/spf13/cast"
	"golang.org/x/net/http2"
)

const (
	Query     = "query"
	Path      = "path"
	Form      = "form"
	UrlEncode = "urlencoded"
	Multipart = "multipart"
	Body      = "body"
	Head      = "header"
	Cookies   = "cookies"
)

const DefaultTimeout = 5 * time.Second

type Client struct {
	Protocol string
	Host     string
	Port     uint16
	Timeout  time.Duration
}

type MultipartFile struct {
	Filename string
	Header   textproto.MIMEHeader

	Data io.Reader
}

func (f *Client) Invoke(ctx context.Context, req interface{}) (ResponseBind, error) {
	request, ok := req.(*http.Request)
	if !ok {
		request2, err := f.newRequest(ctx, req)
		if err != nil {
			return nil, err
		}
		request = request2
	}

	// 从 context 中获取 RequestConfig 并应用到 HTTP 请求
	if config := GetRequestConfigFromContext(ctx); config != nil {
		applyRequestConfig(request, config)
	}

	httpClient := ClientFromContext(ctx)
	if httpClient == nil {
		timeout := f.Timeout
		// 如果 RequestConfig 中有 timeout，使用它
		if config := GetRequestConfigFromContext(ctx); config != nil && config.Timeout != nil {
			timeout = *config.Timeout
		}
		httpClient = GetShortConnClientContext(ctx, timeout)
	}

	if ctxReq, ok := ctx.Value(RequestContextKey).(*http.Request); ok {
		otel.GetTextMapPropagator().Inject(ctxReq.Context(), propagation.HeaderCarrier(request.Header))
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	return &Result{
		Response: resp,
	}, nil
}

func (f *Client) newRequest(ctx context.Context, req interface{}) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	method := ""
	if methodDescriber, ok := req.(MethodDescriber); ok {
		method = methodDescriber.Method()
	}

	path := ""
	if pathDescriber, ok := req.(PathDescriber); ok {
		path = pathDescriber.Path()
	}

	request, err := f.newRequestWithContext(ctx, method, f.toUrl(path), req)
	if err != nil {
		return nil, err
	}

	return request, nil
}

// newRequestWithContext 根据结构体字段标签构建HTTP请求
// 这是客户端的核心函数，负责解析结构体字段的in标签，
// 并将字段值绑定到HTTP请求的不同部分（header、query、body等）
// 支持多种数据格式：JSON、表单、multipart、URL编码等
func (f *Client) newRequestWithContext(ctx context.Context, method string, rawUrl string, v interface{}) (*http.Request, error) {
	header := http.Header{}
	// 从上下文获取语言设置，支持国际化
	lang, ok := ctx.Value(CurrentLangHeader()).(string)
	if ok {
		header.Add(CurrentLangHeader(), lang)
	} else {
		header.Add(CurrentLangHeader(), ginx.i18nLang)
	}

	// 处理空请求体的情况
	if v == nil {
		req, err := http.NewRequestWithContext(ctx, method, rawUrl, nil)
		if err != nil {
			return nil, err
		}
		req.Header = header
		return req, nil
	}

	// 初始化各种参数容器
	query := url.Values{}               // 查询参数
	cookies := url.Values{}             // Cookie参数
	body := new(bytes.Buffer)           // 请求体
	writer := multipart.NewWriter(body) // multipart表单写入器

	// 获取反射值和类型信息
	rv, ok := v.(reflect.Value)
	if !ok {
		rv = reflect.ValueOf(v)
	}
	rv = reflectx.Indirect(rv)
	rt := reflectx.Deref(reflect.TypeOf(v))

	var closeWriter bool // 标记是否需要关闭multipart写入器

	// 遍历结构体字段，根据in标签进行参数绑定
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if in, ok := field.Tag.Lookup("in"); ok {
			// 确定参数名称：优先使用name标签，其次json标签，最后使用小写字段名
			name := field.Tag.Get("name")
			if name == "" {
				name = field.Tag.Get("json")
			}
			if name == "" {
				name = strings.ToLower(field.Name[:1]) + field.Name[1:]
			}

			// 根据in标签值进行不同的参数绑定
			switch in {
			case Head: // HTTP头部
				header[textproto.CanonicalMIMEHeaderKey(name)] = append(header[textproto.CanonicalMIMEHeaderKey(name)], cast.ToString(rv.Field(i).Interface()))
			case Cookies: // Cookie
				cookies[name] = append(cookies[name], cast.ToString(rv.Field(i).Interface()))
			case Query: // URL查询参数
				query.Add(name, cast.ToString(rv.Field(i).Interface()))
			case Path: // 路径参数（替换URL中的占位符）
				rawUrl = strings.Replace(rawUrl, fmt.Sprintf(":%s", name), cast.ToString(rv.Field(i).Interface()), -1)
			case Body: // JSON请求体
				data, _ := json.Marshal(rv.Field(i).Interface())
				body.Write(data)
				header.Set("Content-Type", MineApplicationJson)
			case UrlEncode: // URL编码表单数据
				query.Add(name, rv.Field(i).String())
				header.Set("Content-Type", MineApplicationUrlencoded)
			case Form, Multipart: // 表单或multipart数据
				switch typ := rv.Field(i).Interface().(type) {
				case MultipartFile: // 单个文件上传
					part, err := writer.CreateFormFile(name, typ.Filename)
					if err != nil {
						return nil, err
					}

					if _, err := io.Copy(part, typ.Data); err != nil {
						return nil, err
					}
				case []MultipartFile: // 多文件上传
					for _, f := range typ {
						part, err := writer.CreateFormFile(name, f.Filename)
						if err != nil {
							return nil, err
						}
						if _, err := io.Copy(part, f.Data); err != nil {
							return nil, err
						}
					}
				default: // 普通表单字段
					writer.WriteField(name, cast.ToString(rv.Field(i).Interface()))
				}
				header.Set("Content-Type", writer.FormDataContentType())
				closeWriter = true // 标记需要关闭写入器
			}
		}
	}

	// 关闭multipart写入器（关键！否则内容长度会不匹配导致panic）
	if closeWriter {
		//  necessary !!!!  otherwise the length of the content is shorter than the length of the body !!!! panic
		if writer != nil {
			err := writer.Close()
			if err != nil {
				return nil, err
			}
		}
	}

	// 处理查询参数：如果是URL编码格式，放入body；否则放入URL
	var rawQuery string
	if len(query) > 0 {
		if header.Get("Content-Type") == MineApplicationUrlencoded {
			body = bytes.NewBufferString(query.Encode())
		} else {
			rawQuery = query.Encode()
		}
	}

	// 构建最终的HTTP请求
	req, err := http.NewRequestWithContext(ctx, method, rawUrl, body)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = rawQuery
	req.Header = header

	// 添加Cookie（按名称排序以确保一致性）
	if n := len(cookies); n > 0 {
		names := make([]string, n)
		i := 0
		for name := range cookies {
			names[i] = name
			i++
		}
		sort.Strings(names)

		for _, name := range names {
			values := cookies[name]
			for i := range values {
				req.AddCookie(&http.Cookie{
					Name:  name,
					Value: values[i],
				})
			}
		}
	}

	return req, nil
}

func (f *Client) toUrl(path string) string {
	protocol := f.Protocol
	if protocol == "" {
		protocol = "http"
	}
	url := fmt.Sprintf("%s://%s", protocol, f.Host)
	if f.Port > 0 {
		url = fmt.Sprintf("%s:%d", url, f.Port)
	}
	return url + path
}

func GetShortConnClientContext(ctx context.Context, clientTimeout time.Duration) *http.Client {
	t := DefaultHttpTransportFromContext(ctx)

	if t != nil {
		t = t.Clone()
	} else {
		t = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 0,
			}).DialContext,
			DisableKeepAlives:     true,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}

	if err := http2.ConfigureTransport(t); err != nil {
		// HTTP/2 configuration failed, continue with HTTP/1.1
		// This is not a fatal error, just log it for debugging
		// Could add logging here if needed: log.Printf("HTTP/2 configuration failed: %v", err)
	}

	timeout := DefaultClientTimeout(ctx)
	if timeout != nil {
		clientTimeout = *timeout
	} else {
		clientTimeout = DefaultTimeout
	}

	client := &http.Client{
		Timeout: clientTimeout,
		// Transport: otelhttp.NewTransport(t),
		Transport: t,
	}

	return client
}

type Result struct {
	Response *http.Response
}

func (r *Result) StatusCode() int {
	if r.Response != nil {
		return r.Response.StatusCode
	}
	return 0
}

func (r *Result) Bind(body interface{}) error {
	defer func() {
		if r.Response != nil && r.Response.Body != nil {
			r.Response.Body.Close()
		}
	}()

	data, err := io.ReadAll(r.Response.Body)
	if err != nil {
		return err
	}
	if isOk(r.Response.StatusCode) {
		return json.Unmarshal(data, body)
	}
	statusErr := &statuserror.StatusErr{}
	err = json.Unmarshal(data, statusErr)

	// 如果解析失败或返回空结构体，返回 RemoteHTTPError
	if err != nil || statusErr.K == "" && statusErr.ErrorCode == 0 && statusErr.Message == "" {
		return NewRemoteHTTPError(r.Response.StatusCode, r.Response.Header, data, r.Response.Header.Get("Content-Type"))
	}

	return statusErr
}

func isOk(code int) bool {
	return code >= http.StatusOK && code < http.StatusMultipleChoices
}
