package ginx

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/shrewx/ginx/pkg/trace"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

var (
	server *Server
	once   sync.Once
	conf   = &Ginx{
		Command: &cobra.Command{},
		i18n:    I18nZH,
	}
	confFile string
)

type Ginx struct {
	*cobra.Command
	i18n string
}

// Parse function is parse the config file
// support yaml, json, toml, env
func Parse(conf interface{}) {
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
}

// AddCommand function is add command to cli
func AddCommand(cmds ...*cobra.Command) {
	conf.Command.AddCommand(cmds...)
}

// Launch function is program entry， you can start a server like this:
//	ginx.Launch(func(cmd *cobra.Command, args []string) {
//
//		ginx.Parse(&ServerConfig)
//
//		ginx.RunServer(ServerConfig, router.V0Router)
//	})
func Launch(run func(cmd *cobra.Command, args []string)) {
	conf.Command.Run = run
	conf.Command.Flags().StringVarP(&confFile, "config", "f", "config.yml", "define server conf file path")
	if err := conf.Execute(); err != nil {
		panic(err)
	}
}

// SetI18n function is set the language of message
func SetI18n(language string) {
	switch language {
	case I18nEN:
		conf.i18n = I18nEN
	default:
		conf.i18n = I18nZH
	}
}

func RunServer(config *ServerConfig, r *GinRouter) {
	if config == nil {
		config = NewOptions()
	}

	// release mode
	if config.Release {
		gin.SetMode(gin.ReleaseMode)
	}

	// trace agent
	agent := trace.NewAgent(config.Name, config.TraceEndpoint, config.TraceExporter)
	if err := agent.Init(); err != nil {
		panic(err)
	}

	// init engine
	instance().engine = initGinEngine(r, agent)

	// listen server
	instance().spin(config)
}

type Server struct {
	engine          *gin.Engine
	signalWaiter    func(err chan error) error
	graceCloseHooks []Callback
}

type Callback func(ctx context.Context)

func AddFinishHook(callback ...Callback) {
	instance().graceCloseHooks = append(server.graceCloseHooks, callback...)
}

func instance() *Server {
	once.Do(func() {
		server = new(Server)
	})

	return server
}

func (s *Server) spin(conf *ServerConfig) {
	errCh := make(chan error)
	go func() {
		errCh <- s.run(conf)
	}()

	signalWaiter := waitSignal
	if s.signalWaiter != nil {
		signalWaiter = s.signalWaiter
	}

	if err := signalWaiter(errCh); err != nil {
		logrus.Errorf("receive close signal: error=%s", err.Error())
		ctx, cancel := context.WithTimeout(context.Background(), conf.ExitWaitTimeout)
		defer cancel()
		s.graceClose(ctx)
		return
	}
}

func (s *Server) run(conf *ServerConfig) (err error) {
	if conf.Https && (conf.CertFile == "" || conf.KeyFile == "") {
		panic("use https but cert file or key file not set")
	}
	addr := conf.Host + ":" + strconv.Itoa(conf.Port)
	server := &http.Server{Addr: addr, Handler: s.engine}

	if conf.Https {
		server.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
			MaxVersion:         conf.MaxVersion,
			MinVersion:         conf.MinVersion,
			CipherSuites:       conf.CipherSuites,
		}
		err = server.ListenAndServeTLS(conf.CertFile, conf.KeyFile)
	} else {
		err = server.ListenAndServe()
	}

	if err != nil {
		return err
	}

	return err
}

func (s *Server) graceClose(ctx context.Context) {
	for _, hook := range s.graceCloseHooks {
		hook(ctx)
	}
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
