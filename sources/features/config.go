package features

import (
	"ximanager/sources/platform"
)

type FeatureConfig struct {
	UnleashAPIURL    string
	UnleashInstanceID string
	UnleashAppName   string
	RefreshInterval  int
}

func NewFeatureConfig() *FeatureConfig {
	return &FeatureConfig{
		UnleashAPIURL:     platform.Get("UNLEASH_API_URL", "http://ximanager-unleash:4242/api/"),
		UnleashInstanceID: platform.Get("UNLEASH_INSTANCE_ID", "ximanager"),
		UnleashAppName:    "ximanager",
		RefreshInterval:   platform.GetAsInt("UNLEASH_REFRESH_INTERVAL", 5),
	}
}