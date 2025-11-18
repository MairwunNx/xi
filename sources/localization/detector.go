package localization

import (
	"strings"
	"ximanager/sources/features"
	"ximanager/sources/texting/transform"
	"ximanager/sources/tracing"

	"github.com/pemistahl/lingua-go"
)

const (
	MinTextLengthForDetection = 7
	MaxTextLengthForDetection = 256
)

type LanguageDetector struct {
	detector lingua.LanguageDetector
	features *features.FeatureManager
	log      *tracing.Logger
}

func NewLanguageDetector(features *features.FeatureManager, log *tracing.Logger) *LanguageDetector {
	languages := []lingua.Language{lingua.Russian, lingua.English, lingua.Chinese, lingua.Ukrainian, lingua.Belarusian, lingua.Latvian, lingua.Lithuanian}
	detector := lingua.NewLanguageDetectorBuilder().FromLanguages(languages...).WithPreloadedLanguageModels().Build()

	log.I("Language detector initialized")
	return &LanguageDetector{detector: detector, features: features, log: log}
}

func (x *LanguageDetector) DetectLanguage(text string) string {
	defer tracing.ProfilePoint(x.log, "Language detection completed", "language.detect", "text_length", len(text))()

	if !x.features.IsEnabledOrDefault(features.FeatureLocalizationAuto, true) {
		x.log.D("Language detection disabled by feature flag")
		return "ru" // Именно русский, потому что такой язык был до введения этой фичи.
	}

	cleanText := strings.TrimSpace(text)

	if len(cleanText) < MinTextLengthForDetection {
		x.log.D("Text too short for detection, using English as default", "text_length", len(cleanText), "min_length", MinTextLengthForDetection)
		return "en"
	}

	truncatedText := transform.SmartTruncate(cleanText, MaxTextLengthForDetection)

	if language, exists := x.detector.DetectLanguageOf(truncatedText); exists {
		langCode := x.linguaToCode(language)
		x.log.I("Language detected", "detected_language", langCode)
		return langCode
	}

	x.log.D("Could not detect language, using English as default")
	return "en"
}

func (x *LanguageDetector) linguaToCode(lang lingua.Language) string {
	switch lang {
	case lingua.Russian, lingua.Ukrainian, lingua.Belarusian, lingua.Latvian, lingua.Lithuanian:
		return "ru"
	case lingua.English:
		return "en"
	case lingua.Chinese:
		return "zh"
	default:
		return "en"
	}
}