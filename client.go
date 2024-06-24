package ginx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-courier/reflectx"
	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/spf13/cast"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/net/http2"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"
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

func (f *Client) Invoke(ctx context.Context, req interface{}) (Response, error) {
	request, ok := req.(*http.Request)
	if !ok {
		request2, err := f.newRequest(ctx, req)
		if err != nil {
			return nil, err
		}
		request = request2
	}

	httpClient := ClientFromContext(ctx)
	if httpClient == nil {
		httpClient = GetShortConnClientContext(ctx, f.Timeout)
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

	request = request.WithContext(ctx)

	return request, nil
}

func (f *Client) newRequestWithContext(ctx context.Context, method string, rawUrl string, v interface{}) (*http.Request, error) {
	header := http.Header{}
	// get lang from ctx with LangHeader key
	lang, ok := ctx.Value(LangHeader).(string)
	if ok {
		header.Add(LangHeader, lang)
	} else {
		header.Add(LangHeader, ginx.i18n)
	}

	if v == nil {
		req, err := http.NewRequestWithContext(ctx, method, rawUrl, nil)
		if err != nil {
			return nil, err
		}
		req.Header = header
		return req, nil
	}

	query := url.Values{}
	cookies := url.Values{}
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	rv, ok := v.(reflect.Value)
	if !ok {
		rv = reflect.ValueOf(v)
	}
	rv = reflectx.Indirect(rv)
	rt := reflectx.Deref(reflect.TypeOf(v))

	var closeWriter bool

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if in, ok := field.Tag.Lookup("in"); ok {
			name := field.Tag.Get("name")
			if name == "" {
				name = field.Tag.Get("json")
			}
			if name == "" {
				name = strings.ToLower(field.Name[:1]) + field.Name[1:]
			}
			switch in {
			case Head:
				header[textproto.CanonicalMIMEHeaderKey(name)] = append(header[textproto.CanonicalMIMEHeaderKey(name)], rv.Field(i).String())
			case Cookies:
				cookies[name] = append(cookies[name], rv.Field(i).String())
			case Query:
				query.Add(name, rv.Field(i).String())
			case Path:
				rawUrl = strings.Replace(rawUrl, fmt.Sprintf(":%s", name), cast.ToString(rv.Field(i).Interface()), -1)
			case Body:
				data, _ := json.Marshal(rv.Field(i).Interface())
				body.Write(data)
				header.Set("Content-Type", MineApplicationJson)
			case UrlEncode:
				query.Add(name, rv.Field(i).String())
				header.Set("Content-Type", mime.FormatMediaType(MineApplicationUrlencoded, map[string]string{
					"param": "value",
				}))
			case Form, Multipart:
				switch typ := rv.Field(i).Interface().(type) {
				case MultipartFile:
					part, err := writer.CreateFormFile(name, typ.Filename)
					if err != nil {
						return nil, err
					}

					if _, err := io.Copy(part, typ.Data); err != nil {
						return nil, err
					}
				case []MultipartFile:
					for _, f := range typ {
						part, err := writer.CreateFormFile(name, f.Filename)
						if err != nil {
							return nil, err
						}
						if _, err := io.Copy(part, f.Data); err != nil {
							return nil, err
						}
					}
				default:
					writer.WriteField(name, rv.Field(i).String())
				}
				header.Set("Content-Type", writer.FormDataContentType())
				closeWriter = true
			}
		}
	}

	if closeWriter {
		//  necessary !!!!  otherwise the length of the content is shorter than the length of the body !!!! panic
		err := writer.Close()
		if err != nil {
			return nil, err
		}
	}

	var rawQuery string
	if len(query) > 0 {
		if header.Get("Content-Type") == MineApplicationUrlencoded {
			body = bytes.NewBufferString(query.Encode())
		} else {
			rawQuery = query.Encode()
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawUrl, body)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = rawQuery
	req.Header = header

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

type contextKeyClient struct{}

func ContextWithClient(ctx context.Context, c *http.Client) context.Context {
	return context.WithValue(ctx, contextKeyClient{}, c)
}

func ClientFromContext(ctx context.Context) *http.Client {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(contextKeyClient{}).(*http.Client); ok {
		return c
	}
	return nil
}

type contextKeyDefaultHttpTransport struct{}

func ContextWithDefaultHttpTransport(ctx context.Context, t *http.Transport) context.Context {
	return context.WithValue(ctx, contextKeyDefaultHttpTransport{}, t)
}

func DefaultHttpTransportFromContext(ctx context.Context) *http.Transport {
	if ctx == nil {
		return nil
	}
	if t, ok := ctx.Value(contextKeyDefaultHttpTransport{}).(*http.Transport); ok {
		return t
	}
	return nil
}

type clientTimeout struct{}

func SetClientTimeout(ctx context.Context, timeout time.Duration) context.Context {
	if timeout < 0 {
		timeout = DefaultTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, clientTimeout{}, timeout)
}

func DefaultClientTimeout(ctx context.Context) *time.Duration {
	if ctx == nil {
		return nil
	}
	if t, ok := ctx.Value(contextKeyDefaultHttpTransport{}).(time.Duration); ok {
		if t < 0 {
			return nil
		}
		return &t
	}

	return nil
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
		panic(err)
	}

	timeout := DefaultClientTimeout(ctx)
	if timeout != nil {
		clientTimeout = *timeout
	}

	client := &http.Client{
		Timeout:   clientTimeout,
		Transport: otelhttp.NewTransport(t),
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
	if err != nil {
		return errors.New("failed to bind body or err")
	}

	return statusErr
}

func isOk(code int) bool {
	return code >= http.StatusOK && code < http.StatusMultipleChoices
}

type MultipartFile struct {
	Filename string
	Header   textproto.MIMEHeader

	Data io.Reader
}
