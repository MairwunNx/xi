package external

import (
	"fmt"
	"net/http"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Outsiders struct {
	log    *tracing.Logger
	config *OutsidersConfig
	ss     *http.Server
	sms    *http.Server
	as     *http.Server
}

func NewOutsiders(log *tracing.Logger, config *OutsidersConfig) *Outsiders {
	systemRegistry := prometheus.NewRegistry()
	
	systemRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewBuildInfoCollector(),
	)

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
		sms: &http.Server{
			Addr: fmt.Sprintf(":%d", config.SystemMetricsPort),
			Handler: platform.Curry(http.NewServeMux, func(m *http.ServeMux) {
				m.Handle("/metrics", promhttp.HandlerFor(systemRegistry, promhttp.HandlerOpts{}))
			}),
		},
		as: &http.Server{
			Addr: fmt.Sprintf(":%d", config.ApplicationMetricsPort),
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

func (x *Outsiders) systemMetrics() {
	x.log.I("System metrics server is starting", tracing.OutsiderKind, "system_metrics", "port", x.config.SystemMetricsPort)

	if err := x.sms.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		x.log.F("Failed to start system metrics server", tracing.OutsiderKind, "system_metrics", tracing.InnerError, err)
	}
}

func (x *Outsiders) applicationMetrics() {
	x.log.I("Application metrics server is starting", tracing.OutsiderKind, "application_metrics", "port", x.config.ApplicationMetricsPort)

	if err := x.as.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		x.log.F("Failed to start application metrics server", tracing.OutsiderKind, "application_metrics", tracing.InnerError, err)
	}
}

func startuphandler(log *tracing.Logger, w http.ResponseWriter, r *http.Request) {
	log.I("Outsider service got a ping", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"ximanager"}`))
}