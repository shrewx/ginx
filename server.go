package ginx

import (
	"crypto/tls"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/trace"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"strconv"
)

var I18n = I18nZH

type Server struct {
	ServerConfig `yaml:"server_config"`

	engine *gin.Engine
	g      errgroup.Group
}

type ServerConfig struct {
	Name    string `yaml:"name"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Https   bool   `yaml:"https"`
	Release bool   `yaml:"release"`

	SSLConfig `yaml:"ssl_config"`

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

func (s *Server) init() {
	if s.MaxVersion == 0 {
		s.MinVersion = tls.VersionTLS12
	}
	if s.MaxVersion == 0 {
		s.MaxVersion = tls.VersionTLS13
	}
	if len(s.CipherSuites) == 0 {
		s.CipherSuites = []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		}
	}
}

func RunServer(s *Server, r *GinRouter) {
	s.init()

	// release mode
	if s.Release {
		gin.SetMode(gin.ReleaseMode)
	}

	// trace agent
	agent := trace.NewAgent(s.Name, s.TraceEndpoint, s.TraceExporter)
	if err := agent.Init(); err != nil {
		panic(err)
	}

	// init engine
	s.engine = initGinEngine(r, agent)

	s.run()

	s.loop()
}

func (s *Server) run() {
	if s.Https && (s.CertFile == "" || s.KeyFile == "") {
		panic("use https but cert file or key file not set")
	}

	s.g.Go(func() error {
		if s.Https {
			return s.runHttps()
		} else {
			return s.runHttp()
		}
	})
}

func (s *Server) runHttps() (err error) {
	addr := s.Host + ":" + strconv.Itoa(s.Port)
	server := &http.Server{Addr: addr, Handler: s.engine}
	server.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		MaxVersion:         s.MaxVersion,
		MinVersion:         s.MinVersion,
	}
	server.TLSConfig.CipherSuites = s.CipherSuites
	err = server.ListenAndServeTLS(s.CertFile, s.KeyFile)
	if err != nil {
		return err
	}
	return
}

func (s *Server) runHttp() (err error) {
	addr := s.Host + ":" + strconv.Itoa(s.Port)
	server := &http.Server{Addr: addr, Handler: s.engine}
	err = server.ListenAndServe()
	if err != nil {
		return err
	}
	return
}

func (s *Server) loop() {
	if err := s.g.Wait(); err != nil {
		log.Fatal("loop error:", err)
	}
}

func SetI18n(language string) {
	switch language {
	case I18nEN:
		I18n = I18nEN
	default:
		I18n = I18nZH
	}
}
