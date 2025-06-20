package main

import (
	"context"
	"ximanager/sources/artificial"
	"ximanager/sources/balancer"
	"ximanager/sources/external"
	"ximanager/sources/network"
	"ximanager/sources/persistence"
	"ximanager/sources/repository"
	"ximanager/sources/telegram"
	"ximanager/sources/throttler"
	"ximanager/sources/tracing"

	"go.uber.org/fx"
)

var (
	version = "0.0.0"
	buildTime = "1970-01-01"
)

func main() {
	fx.New(
		tracing.Module,
		external.Module,
		network.Module,
		persistence.Module,
		repository.Module,
		throttler.Module,
		artificial.Module,
		balancer.Module,
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