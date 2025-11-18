package localization

import (
	"ximanager/sources/platform"
)

type LocalizationConfig struct {
	DefaultLanguage string
	SupportedLanguages []string
}

func NewLocalizationConfig() *LocalizationConfig {
	return &LocalizationConfig{
		DefaultLanguage:    platform.Get("LOCALIZATION_DEFAULT_LANG", "ru"),
		SupportedLanguages: []string{"ru", "en", "zh"},
	}
}