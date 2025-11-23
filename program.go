package main

import (
	"context"
	"os"
	"time"
	"ximanager/sources/artificial"
	"ximanager/sources/configuration"
	"ximanager/sources/external"
	"ximanager/sources/features"
	"ximanager/sources/localization"
	"ximanager/sources/metrics"
	"ximanager/sources/metrics/collector"
	"ximanager/sources/network"
	"ximanager/sources/persistence"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/telegram"
	"ximanager/sources/texting/format"
	"ximanager/sources/texting/personality"
	"ximanager/sources/throttler"
	"ximanager/sources/tracing"

	"go.uber.org/fx"
)

var (
	version = "0.0.0"
	buildTime = "1970-01-01"
	startTime = time.Now()
)

func main() {
	platform.SetAppManifest(version, buildTime, startTime)

	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
				time.Local = loc
		}
	}

	fx.New(
		metrics.Module,
		collector.Module,
		tracing.Module,
		configuration.Module,
		external.Module,
		network.Module,
		persistence.Module,
		repository.Module,
		throttler.Module,
		features.Module,
		localization.Module,
		format.Module,
		personality.Module,
		artificial.Module,
		telegram.Module,
	
		fx.Invoke(func(lc fx.Lifecycle, log *tracing.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					log.I("Xi Manager started successfully", "version", version, "build_time", buildTime)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					log.I("Xi Manager stopped", "version", version, "build_time", buildTime)
					return nil
				},
			})
		}),
	).Run()
}