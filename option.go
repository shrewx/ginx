package ginx

import (
	"crypto/tls"
	"time"
)

type ServerConfig struct {
	Name    string `yaml:"name"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Https   bool   `yaml:"https"`
	Release bool   `yaml:"release"`

	SSLConfig `yaml:"ssl_config"`

	ExitWaitTimeout time.Duration `yaml:"exit_wait_timeout"`

	// trace
	TraceEndpoint string `yaml:"trace_endpoint"`
	TraceExporter string `yaml:"trace_exporter"`
}

type SSLConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`

	MaxVersion   uint16   `yaml:"max_version"`
	MinVersion   uint16   `yaml:"min_version"`
	CipherSuites []uint16 `yaml:"cipher_suites"`
}

type Option func(s *ServerConfig)

func NewOptions(options ...Option) *ServerConfig {
	conf := &ServerConfig{
		Port:            80,
		ExitWaitTimeout: 5 * time.Second,
		SSLConfig: SSLConfig{
			MaxVersion: tls.VersionTLS13,
			MinVersion: tls.VersionTLS12,
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
		},
	}

	for _, op := range options {
		op(conf)
	}

	return conf
}

func WithName(name string) Option {
	return func(s *ServerConfig) {
		s.Name = name
	}
}

func WithPort(port int) Option {
	return func(s *ServerConfig) {
		s.Port = port
	}
}

func WithHost(host string) Option {
	return func(s *ServerConfig) {
		s.Host = host
	}
}

func WithHttps() Option {
	return func(s *ServerConfig) {
		s.Https = true
	}
}

func WithSSLFile(certFile, keyFile string) Option {
	return func(s *ServerConfig) {
		s.CertFile = certFile
		s.KeyFile = keyFile
	}
}

func WithGraceExitTime(timeout time.Duration) Option {
	return func(s *ServerConfig) {
		s.ExitWaitTimeout = timeout
	}
}
