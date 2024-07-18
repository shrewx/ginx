package conf

import (
	"crypto/tls"
)

type Server struct {
	ID              string `yaml:"id"`
	Name            string `yaml:"name"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Https           bool   `yaml:"https"`
	Release         bool   `yaml:"release"`
	ExitWaitTimeout int    `yaml:"exit_wait_timeout"`

	TLS       `yaml:"tls"`
	Trace     `yaml:"trace"`
	Discovery `yaml:"discovery"`
}

type TLS struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`

	MaxVersion   uint16   `yaml:"max_version"`
	MinVersion   uint16   `yaml:"min_version"`
	CipherSuites []uint16 `yaml:"cipher_suites"`
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
