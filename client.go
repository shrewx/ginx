package ginx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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

const DefaultTimeout = 60 * time.Second

// Client 底层 HTTP 客户端
type Client struct {
	config ClientConfig
}

// NewClient 创建新的客户端
func NewClient(config ClientConfig) *Client {
	if config.Protocol == "" {
		config.Protocol = "http"
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	return &Client{config: config}
}

type MultipartFile struct {
	Filename string
	Header   textproto.MIMEHeader

	Data io.Reader
}

// Invoke 执行请求（核心方法）
func (c *Client) Invoke(ctx context.Context, req interface{}, opts ...RequestOption) (ResponseBind, error) {
	// 1. 构建请求配置
	requestConfig := buildRequestConfig(opts...)

	// 2. 构建 HTTP 请求
	// 如果 req 已经是 *http.Request，直接使用；否则通过 newRequest 构建
	var httpReq *http.Request
	var err error
	if httpRequest, ok := req.(*http.Request); ok {
		httpReq = httpRequest
	} else {
		httpReq, err = c.newRequest(ctx, req)
		if err != nil {
			return nil, err
		}
	}

	// 3. 应用请求配置到 HTTP 请求
	applyRequestConfig(httpReq, requestConfig)

	// 4. 获取或创建 HTTP Client
	httpClient := getHTTPClient(&c.config, requestConfig, ctx)

	// 5. 注入 OpenTelemetry 追踪信息
	if ctxReq, ok := ctx.Value(RequestContextKey).(*http.Request); ok {
		otel.GetTextMapPropagator().Inject(ctxReq.Context(), propagation.HeaderCarrier(httpReq.Header))
	}

	// 6. 执行请求
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	return &Result{Response: resp}, nil
}

// buildRequestConfig 构建请求配置
func buildRequestConfig(opts ...RequestOption) *RequestConfig {
	config := NewRequestConfig()
	config.Apply(opts...)
	return config
}

func (c *Client) newRequest(ctx context.Context, req interface{}) (*http.Request, error) {
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

	request, err := c.newRequestWithContext(ctx, method, c.toUrl(path), req)
	if err != nil {
		return nil, err
	}

	return request, nil
}

// newRequestWithContext 根据结构体字段标签构建HTTP请求
// 这是客户端的核心函数，负责解析结构体字段的in标签，
// 并将字段值绑定到HTTP请求的不同部分（header、query、body等）
// 支持多种数据格式：JSON、表单、multipart、URL编码等
func (c *Client) newRequestWithContext(ctx context.Context, method string, rawUrl string, v interface{}) (*http.Request, error) {
	header := http.Header{}
	// 从上下文获取语言设置，支持国际化
	lang, ok := ctx.Value(CurrentLangHeader()).(string)
	if ok {
		header.Add(CurrentLangHeader(), lang)
	} else {
		header.Add(CurrentLangHeader(), I18nZH)
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

func (c *Client) toUrl(path string) string {
	protocol := c.config.Protocol
	if protocol == "" {
		protocol = "http"
	}
	url := fmt.Sprintf("%s://%s", protocol, c.config.Host)
	if c.config.Port > 0 {
		url = fmt.Sprintf("%s:%d", url, c.config.Port)
	}
	return url + path
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
