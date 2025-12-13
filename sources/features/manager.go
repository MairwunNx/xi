package features

import (
	"context"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/tracing"

	"github.com/Unleash/unleash-client-go/v4"
)

const (
	FeatureLocalizationAuto          = "general/localization/auto"
	FeatureMessageSummarization      = "dialer/context/message/summarization"
	FeatureClusterSummarization      = "dialer/context/cluster/summarization"
	FeatureResponseLengthDetection   = "dialer/response/length-detection"
	FeatureFeedbackButtons           = "dialer/response/feedback-buttons"
	FeaturePersonalizationExtraction = "dialer/personalization/extraction"
)

type FeatureManager struct {
	client *unleash.Client
	log    *tracing.Logger
}

func NewFeatureManager(config *configuration.Config, log *tracing.Logger) (*FeatureManager, error) {
	client, err := unleash.NewClient(
		unleash.WithUrl(config.Features.UnleashAPIURL),
		unleash.WithAppName(config.Features.UnleashAppName),
		unleash.WithInstanceId(config.Features.UnleashInstanceID),
		unleash.WithRefreshInterval(time.Duration(config.Features.RefreshInterval)*time.Second),
		unleash.WithListener(&unleashListener{log: log}),
	)

	if err != nil {
		log.E("Failed to initialize Unleash client", tracing.InnerError, err)
		return nil, err
	}

	log.I("Unleash client initialized successfully", "api_url", config.Features.UnleashAPIURL, "app_name", config.Features.UnleashAppName, "instance_id", config.Features.UnleashInstanceID, "refresh_interval", config.Features.RefreshInterval)
	return &FeatureManager{client: client, log: log}, nil
}

func (x *FeatureManager) IsEnabled(featureName string) bool {
  defer tracing.ProfilePoint(x.log, "Unleash feature requested completed", "unleash.feature.requested", "feature_name", featureName)()
	return x.client.IsEnabled(featureName)
}

func (x *FeatureManager) IsEnabledOrDefault(featureName string, defaultValue bool) bool {
  defer tracing.ProfilePoint(x.log, "Unleash feature requested completed", "unleash.feature.requested", "feature_name", featureName, "default_value", defaultValue)()
	return x.client.IsEnabled(featureName, unleash.WithFallback(defaultValue))
}

func (x *FeatureManager) Close() error {
  defer tracing.ProfilePoint(x.log, "Unleash client closed", "unleash.close")()
  return x.client.Close()
}

type unleashListener struct {
	log *tracing.Logger
}

func (x *unleashListener) OnReady() {
	x.log.I("Unleash client ready")
}

func (x *unleashListener) OnError(err error) {
	x.log.E("Unleash client error", tracing.InnerError, err)
}

func (x *unleashListener) OnWarning(warning error) {
	x.log.W("Unleash client warning", tracing.InnerError, warning)
}

func (x *unleashListener) OnRegistered(payload unleash.ClientData) {
	x.log.I("Unleash client registered", "instance_id", payload.InstanceID)
}

func (x *unleashListener) OnCount(name string, enabled bool) {}
func (x *unleashListener) OnSent(payload unleash.MetricsData) {}
func (x *FeatureManager) OnStop(ctx context.Context) error { return x.Close() }
