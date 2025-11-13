package conf

import (
	"crypto/tls"
)

type Server struct {
	// 服务ID
	ID string `yaml:"id" env:"SERVER_ID"`
	// 服务名称
	Name string `yaml:"name" env:"SERVER_NAME"`
	// 服务主机
	Host string `yaml:"host" env:"SERVER_HOST"`
	// 服务端口
	Port int `yaml:"port" env:"SERVER_PORT"`
	// 是否启用HTTPS
	Https bool `yaml:"https" env:"SERVER_HTTPS"`
	// 退出等待超时时间(秒)
	ExitWaitTimeout int `yaml:"exit_wait_timeout" env:"SERVER_EXIT_WAIT_TIMEOUT"`

	// 是否打印请求参数
	ShowParams bool `yaml:"show_params" env:"SERVER_SHOW_PARAMS"`

	Log *Log `yaml:"log" env:"SERVER_LOG"`

	I18N *I18N `yaml:"i18n" env:"SERVER_I18N"`

	TLS       `yaml:"tls"`
	Trace     `yaml:"trace"`
	Discovery `yaml:"discovery"`
}

type TLS struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" env:"SERVER_TLS_INSECURE_SKIP_VERIFY"`
	CertFile           string `yaml:"cert_file" env:"SERVER_TLS_CERT_FILE"`
	KeyFile            string `yaml:"key_file" env:"SERVER_TLS_KEY_FILE"`

	MaxVersion   uint16   `yaml:"max_version" env:"SERVER_TLS_MAX_VERSION"`
	MinVersion   uint16   `yaml:"min_version" env:"SERVER_TLS_MIN_VERSION"`
	CipherSuites []uint16 `yaml:"cipher_suites" env:"SERVER_TLS_CIPHER_SUITES"`
}

type Trace struct {
	// trace
	TraceEndpoint string `yaml:"trace_endpoint"`
	TraceExporter string `yaml:"trace_exporter"`
}

type Discovery struct {
	Address   string   `yaml:"address"`
	Tags      []string `yaml:"tags"`
	HeathPath string   `yaml:"heath_path"`

	Timeout        int `yaml:"timeout"`
	Interval       int `yaml:"interval"`
	DeregisterTime int `yaml:"deregister_time"`
}

type I18N struct {
	// 可支持语言(en/zh)
	Langs []string `yaml:"langs"`
	// 配置类型(toml/json/yaml)
	UnmarshalType string `yaml:"unmarshal_type"`
	// 配置路径列表（支持多路径）
	Paths []string `yaml:"paths"`
}

type Option func(s *Server)

func NewOptions(options ...Option) *Server {
	conf := &Server{
		Host:            "127.0.0.1",
		Port:            80,
		ExitWaitTimeout: 5,
		TLS: TLS{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			},
			InsecureSkipVerify: true,
		},
	}

	for _, op := range options {
		op(conf)
	}

	return conf
}

func WithName(name string) Option {
	return func(s *Server) {
		s.Name = name
	}
}

func WithHostPorts(host string, port int) Option {
	return func(s *Server) {
		s.Host = host
		s.Port = port
	}
}

func WithTLS(certFile, keyFile string) Option {
	return func(s *Server) {
		s.Https = true
		s.CertFile = certFile
		s.KeyFile = keyFile
	}
}

func WithGraceExitTime(timeout int) Option {
	return func(s *Server) {
		s.ExitWaitTimeout = timeout
	}
}

func WithTrace(endpoint, exporter string) Option {
	return func(s *Server) {
		s.TraceEndpoint = endpoint
		s.TraceExporter = exporter
	}
}
