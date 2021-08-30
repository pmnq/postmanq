package webservice

import (
	"context"
	"errors"
	stdLog "log"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/alexliesenfeld/health"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

const defaultWSAddr = ":1080"

type service struct {
	Debug  bool   `yaml:"debug"`
	WSAddr string `yaml:"wsAddr"`

	routes *http.ServeMux
	server *http.Server
}

func Inst() common.SendingService {
	return new(service)
}

func (s *service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err != nil {
		logger.All().ErrWithErr(err, "can't unmarshal logger config")
	}

	if s.WSAddr == "" {
		s.WSAddr = defaultWSAddr
	}

	s.routes = http.NewServeMux()

	if s.Debug {
		for n, f := range map[string]func(http.ResponseWriter, *http.Request){
			"/debug/pprof/":        pprof.Index,
			"/debug/pprof/cmdline": pprof.Cmdline,
			"/debug/pprof/profile": pprof.Profile,
			"/debug/pprof/symbol":  pprof.Symbol,
			"/debug/pprof/trace":   pprof.Trace,
		} {
			s.routes.HandleFunc(n, f)
		}
	}

	checker := health.NewChecker(
		health.WithCacheDuration(1*time.Second),
		health.WithTimeout(10*time.Second),
	)
	s.routes.Handle("/health", health.NewHandler(checker))

	s.routes.Handle("/metrics", promhttp.Handler())

	s.server = &http.Server{
		Addr:     s.WSAddr,
		Handler:  s.routes,
		ErrorLog: stdLog.New(log.Logger, "", stdLog.Llongfile),
	}
}

func (s *service) Event(_ *common.SendEvent) bool {
	return true
}

func (s *service) OnRun() {
	go func() {
		logger.All().Debug("web server has been started on addr %s successful", s.WSAddr)
		err := s.server.ListenAndServe()
		s.server = nil
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		if err != nil {
			logger.All().FailExitWithErr(err, "ws error")
		}
	}()
}

func (s *service) OnFinish() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if s.server == nil {
		return
	}

	if err := s.server.Shutdown(ctx); err != nil {
		logger.All().ErrWithErr(err, "server shutdown error")
	}

	logger.All().Debug("web server has been shut down successful")
}
