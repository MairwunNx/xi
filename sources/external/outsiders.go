package external

import (
	"fmt"
	"net/http"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Outsiders struct {
	log    *tracing.Logger
	config *OutsidersConfig
	ss     *http.Server
	ms     *http.Server
}

func NewOutsiders(log *tracing.Logger, config *OutsidersConfig) *Outsiders {
	return &Outsiders{
		log:    log,
		config: config,
		ss: &http.Server{
			Addr: fmt.Sprintf(":%d", config.StartupPort),
			Handler: platform.Curry(http.NewServeMux, func(m *http.ServeMux) {
				m.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
					startuphandler(log, w, r)
				})
			}),
		},
		ms: &http.Server{
			Addr: fmt.Sprintf(":%d", config.MetricsPort),
			Handler: platform.Curry(http.NewServeMux, func(m *http.ServeMux) {
				m.Handle("/metrics", promhttp.Handler())
			}),
		},
	}
}

func (x *Outsiders) startup() {
	x.log.I("Startup server is starting", tracing.OutsiderKind, "startup", "port", x.config.StartupPort)

	if err := x.ss.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		x.log.F("Failed to start startup server", tracing.OutsiderKind, "startup", tracing.InnerError, err)
	}
}

func (x *Outsiders) metrics() {
	x.log.I("Metrics server is starting", tracing.OutsiderKind, "metrics", "port", x.config.MetricsPort)

	if err := x.ms.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		x.log.F("Failed to start metrics server", tracing.OutsiderKind, "metrics", tracing.InnerError, err)
	}
}

func startuphandler(log *tracing.Logger, w http.ResponseWriter, r *http.Request) {
	log.I("Outsider service got a ping", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"ximanager"}`))
}
