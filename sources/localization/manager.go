package localization

import (
	"embed"
	"fmt"
	"strings"
	"sync"
	"ximanager/sources/tracing"

	"github.com/BurntSushi/toml"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localesFS embed.FS

type LocalizationManager struct {
	bundle   *i18n.Bundle
	detector *LanguageDetector
	config   *LocalizationConfig
	log      *tracing.Logger
	locbuff  sync.Map
}

func NewLocalizationManager(
	config *LocalizationConfig,
	detector *LanguageDetector,
	log *tracing.Logger,
) (*LocalizationManager, error) {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, lang := range config.SupportedLanguages {
		filename := fmt.Sprintf("locales/active.%s.toml", lang)

		data, err := localesFS.ReadFile(filename)
		if err != nil {
			log.E("Failed to read locale file", "filename", filename, tracing.InnerError, err)
			return nil, fmt.Errorf("failed to read locale file %s: %w", filename, err)
		}

		if _, err := bundle.ParseMessageFileBytes(data, filename); err != nil {
			log.E("Failed to parse locale file", "filename", filename, tracing.InnerError, err)
			return nil, fmt.Errorf("failed to parse locale file %s: %w", filename, err)
		}

		log.I("Loaded locale file", "filename", filename)
	}

	log.I("LocalizationManager initialized successfully")
	return &LocalizationManager{bundle: bundle, detector: detector, config: config, log: log}, nil
}

func (x *LocalizationManager) GetLocalizer(userText string) *i18n.Localizer {
  defer tracing.ProfilePoint(x.log, "Getting localizer completed", "localization.get.localizer", "user_text", userText)()

	localizer := i18n.NewLocalizer(x.bundle, x.detector.DetectLanguage(userText), "en")
	return localizer
}

func (x *LocalizationManager) Localize(localizer *i18n.Localizer, messageID string) string {
	return x.LocalizeTd(localizer, messageID, nil)
}

func (x *LocalizationManager) LocalizeTd(localizer *i18n.Localizer, messageID string, templateData map[string]interface{}) string {
	defer tracing.ProfilePoint(x.log, "Localization completed", "localization.localize")()

	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID, TemplateData: templateData})
	if err != nil {
		x.log.E("Failed to localize message", "message_id", messageID, tracing.InnerError, err)
		return messageID
	}

	return msg
}

func (x *LocalizationManager) LocalizeBy(msg *tgbotapi.Message, messageID string) string {
	return x.LocalizeByTd(msg, messageID, nil)
}

func (x *LocalizationManager) LocalizeByTd(msg *tgbotapi.Message, messageID string, templateData map[string]interface{}) string {
	defer tracing.ProfilePoint(x.log, "LocalizeBy completed", "localization.localize.by")()

	userText := msg.Text
	if userText == "" && msg.Caption != "" {
		userText = msg.Caption
	}

	cleanText := strings.TrimSpace(userText)
	var detectedLang string
	userId := msg.From.ID

	if cleanText != "" {
		detectedLang = x.detector.DetectLanguage(cleanText)
		x.locbuff.Store(userId, detectedLang)
		x.log.D("Locale detected from text and cached", "user_id", userId, "locale", detectedLang)
	} else {
		if cached, ok := x.locbuff.Load(userId); ok {
			detectedLang = cached.(string)
			x.log.D("Locale loaded from cache", "user_id", userId, "locale", detectedLang)
		} else if msg.From.LanguageCode != "" {
			detectedLang = x.mapTelegramLanguageCode(msg.From.LanguageCode)
			x.log.D("Locale loaded from Telegram language code", "user_id", userId, "telegram_code", msg.From.LanguageCode, "locale", detectedLang)
		} else {
			detectedLang = "en"
			x.log.D("No locale information available, using English", "user_id", userId)
		}
	}

	localizer := i18n.NewLocalizer(x.bundle, detectedLang, "en")
	return x.LocalizeTd(localizer, messageID, templateData)
}

func (x *LocalizationManager) mapTelegramLanguageCode(telegramCode string) string {
	lowerCode := strings.ToLower(telegramCode)

	switch {
	case strings.HasPrefix(lowerCode, "ru"), strings.HasPrefix(lowerCode, "uk"), strings.HasPrefix(lowerCode, "be"), strings.HasPrefix(lowerCode, "lv"), strings.HasPrefix(lowerCode, "lt"):
		return "ru"
	case strings.HasPrefix(lowerCode, "en"):
		return "en"
	case strings.HasPrefix(lowerCode, "zh"):
		return "zh"
	default:
		return "en"
	}
}