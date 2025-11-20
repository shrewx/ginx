package ginx

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/shrewx/ginx/pkg/i18nx"

	"github.com/gin-gonic/gin"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/logx"
	"github.com/shrewx/ginx/pkg/service_discovery"
	"github.com/shrewx/ginx/pkg/trace"
	"github.com/spf13/cobra"
)

var (
	confFile string

	ginx = &Ginx{
		Command:  &cobra.Command{},
		i18nLang: I18nZH,
		once:     sync.Once{},
	}

	showParams bool

	langHeaderValue atomic.Value
)

func init() {
	langHeaderValue.Store(LangHeader)
}

type Ginx struct {
	*cobra.Command
	i18nLang string
	server   *Server
	once     sync.Once
}

// Parse function is parse the config file
// support yaml, json, toml, env
func Parse(conf interface{}) {
	if confFile != "" {
		if confFile == DefaultConfig {
			pwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			confFile = filepath.Join(pwd, confFile)
		}

		if err := cleanenv.ReadConfig(confFile, conf); err != nil {
			panic(err)
		}
	} else {
		if err := cleanenv.ReadEnv(conf); err != nil {
			panic(err)
		}
	}
}

// AddCommand function is add command to cli
func AddCommand(cmds ...*cobra.Command) {
	ginx.Command.AddCommand(cmds...)
}

// Launch function is program entry， you can start a server like this:
//
//	ginx.Launch(func(cmd *cobra.Command, args []string) {
//
//		ginx.Parse(&ServerConfig)
//
//		ginx.RunServer(ServerConfig, router.V0Router)
//	})
func Launch(run func(cmd *cobra.Command, args []string)) {
	ginx.Command.Run = run
	ginx.Command.Flags().StringVarP(&confFile, "config", "f", "", "define server conf file path")
	if err := ginx.Execute(); err != nil {
		panic(err)
	}
}

// SetI18n function is set the language of message
// Default I18n is ZH, this will set to cookie
func SetI18n(language string) {
	switch language {
	case I18nEN:
		ginx.i18nLang = I18nEN
	default:
		ginx.i18nLang = I18nZH
	}
}

func RunServer(config *conf.Server, r *GinRouter) {
	if config == nil {
		config = conf.NewOptions()
	}

	showParams = config.ShowParams

	// init log
	if config.Log == nil {
		config.Log = &conf.Log{
			ToStdout: true,
		}
	}
	logx.Load(config.Log)

	// init i18n
	if config.I18N == nil {
		config.I18N = &conf.I18N{}
	}
	i18nx.Load(config.I18N)

	// trace agent
	agent := initTrace(config)

	// init engine
	instance().engine = initGinEngine(r, agent)

	// listen server
	instance().spin(config)
}

type Server struct {
	engine  *gin.Engine
	server  *http.Server
	watcher service_discovery.ServiceDiscovery

	signalWaiter    func(err chan error) error
	graceCloseHooks []Callback
}

type Callback func()

func AddShutdownHook(callback ...Callback) {
	instance().graceCloseHooks = append(ginx.server.graceCloseHooks, callback...)
}

func Register(watcher service_discovery.ServiceDiscovery) {
	instance().watcher = watcher
}

func instance() *Server {
	ginx.once.Do(func() {
		ginx.server = new(Server)
	})

	return ginx.server
}

func (s *Server) spin(conf *conf.Server) {
	errCh := make(chan error)
	go func() {
		errCh <- s.run(conf)
	}()

	// discovery
	s.watch(conf)

	signalWaiter := waitSignal
	if s.signalWaiter != nil {
		signalWaiter = s.signalWaiter
	}

	if err := signalWaiter(errCh); err != nil {
		logx.Errorf("receive close signal: error=%s", err.Error())
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(conf.ExitWaitTimeout)*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
		return
	}
}

func (s *Server) run(conf *conf.Server) (err error) {
	if conf.Https && (conf.TLS.CertFile == "" || conf.TLS.KeyFile == "") {
		panic("use https but cert file or key file not set")
	}
	addr := conf.Host + ":" + strconv.Itoa(conf.Port)
	s.server = &http.Server{Addr: addr, Handler: s.engine}

	// hook
	for _, hook := range s.graceCloseHooks {
		s.server.RegisterOnShutdown(hook)
	}

	if conf.Https {
		s.server.TLSConfig = &tls.Config{
			InsecureSkipVerify: conf.TLS.InsecureSkipVerify,
			MaxVersion:         conf.TLS.MaxVersion,
			MinVersion:         conf.TLS.MinVersion,
			CipherSuites:       conf.TLS.CipherSuites,
		}
		err = s.server.ListenAndServeTLS(conf.TLS.CertFile, conf.TLS.KeyFile)
	} else {
		err = s.server.ListenAndServe()
	}

	if err != nil {
		return err
	}

	return err
}

func (s *Server) watch(conf *conf.Server) error {
	if s.watcher != nil {
		info := service_discovery.ServiceInfo{
			Name:           conf.Name,
			Address:        conf.Discovery.Address,
			Port:           conf.Port,
			Tags:           conf.Discovery.Tags,
			ID:             conf.ID,
			HealthPath:     conf.Discovery.HeathPath,
			Timeout:        conf.Discovery.Timeout,
			Interval:       conf.Discovery.Interval,
			DeregisterTime: conf.Discovery.DeregisterTime,
		}
		info.Default()
		if err := s.watcher.Watch(info); err != nil {
			return err
		}
	}

	return nil
}

func waitSignal(errCh chan error) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)

	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGTERM:
			// force exit
			return errors.New(sig.String())
		case syscall.SIGHUP, syscall.SIGINT:
			// graceful shutdown
			return nil
		}
	case err := <-errCh:

		return err
	}

	return nil
}

func initTrace(conf *conf.Server) *trace.Agent {
	agent := trace.NewAgent(conf.Name, conf.TraceEndpoint, conf.TraceExporter)
	if err := agent.Init(); err != nil {
		panic(err)
	}

	return agent
}

// SetLangHeader 允许用户自定义语言头，如果传入空字符串则回退到默认值
func SetLangHeader(header string) {
	header = strings.TrimSpace(header)
	if header == "" {
		header = LangHeader
	}
	langHeaderValue.Store(header)
}

// CurrentLangHeader 返回当前生效的语言头名称
func CurrentLangHeader() string {
	if value, ok := langHeaderValue.Load().(string); ok && value != "" {
		return value
	}
	return LangHeader
}
