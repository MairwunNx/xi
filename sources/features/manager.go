package features

import (
	"context"
	"time"
	"ximanager/sources/tracing"

	"github.com/Unleash/unleash-client-go/v4"
)

const (
	FeatureMessageSummarization    = "dialer/context/message/summarization"
	FeatureClusterSummarization    = "dialer/context/cluster/summarization"
	FeatureResponseLengthDetection = "dialer/response/length-detection"
)

type FeatureManager struct {
	client *unleash.Client
	config *FeatureConfig
	log    *tracing.Logger
}

func NewFeatureManager(config *FeatureConfig, log *tracing.Logger) (*FeatureManager, error) {
	client, err := unleash.NewClient(
		unleash.WithUrl(config.UnleashAPIURL),
		unleash.WithAppName(config.UnleashAppName),
		unleash.WithInstanceId(config.UnleashInstanceID),
		unleash.WithRefreshInterval(time.Duration(config.RefreshInterval)*time.Second),
		unleash.WithListener(&unleashListener{log: log}),
	)
	
	if err != nil {
		log.E("Failed to initialize Unleash client", tracing.InnerError, err)
		return nil, err
	}
	
	log.I("Unleash client initialized successfully",
		"api_url", config.UnleashAPIURL,
		"app_name", config.UnleashAppName,
		"instance_id", config.UnleashInstanceID,
		"refresh_interval", config.RefreshInterval,
	)
	
	return &FeatureManager{
		client: client,
		config: config,
		log:    log,
	}, nil
}

func (f *FeatureManager) IsEnabled(featureName string) bool {
	return f.client.IsEnabled(featureName)
}

func (f *FeatureManager) IsEnabledDefault(featureName string, defaultValue bool) bool {
	return f.client.IsEnabled(featureName, unleash.WithFallback(defaultValue))
}

func (f *FeatureManager) Close() error {
	f.log.I("Closing Unleash client")
	f.client.Close()
	return nil
}

type unleashListener struct {
	log *tracing.Logger
}

func (l *unleashListener) OnReady() {
	l.log.I("Unleash client ready")
}

func (l *unleashListener) OnError(err error) {
	l.log.E("Unleash client error", tracing.InnerError, err)
}

func (l *unleashListener) OnWarning(warning error) {
	l.log.W("Unleash client warning", tracing.InnerError, warning)
}

func (l *unleashListener) OnCount(name string, enabled bool) {
}

func (l *unleashListener) OnSent(payload unleash.MetricsData) {
}

func (l *unleashListener) OnRegistered(payload unleash.ClientData) {
	l.log.I("Unleash client registered", "instance_id", payload.InstanceID)
}

func (f *FeatureManager) OnStop(ctx context.Context) error {
	return f.Close()
}
