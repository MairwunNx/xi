package personality

import (
	"fmt"
	"ximanager/sources/localization"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type XiPersonality struct {
	localization *localization.LocalizationManager
}

func NewXiPersonality(localization *localization.LocalizationManager) *XiPersonality {
	return &XiPersonality{localization: localization}
}

func (x *XiPersonality) Xiify(msg *tgbotapi.Message, text string) string {
	prefix := x.localization.LocalizeBy(msg, "MsgXiResponse")
	return fmt.Sprintf("%s%s", prefix, text)
}

func (x *XiPersonality) XiifyManual(msg *tgbotapi.Message, text string) string {
	prefix := x.localization.LocalizeBy(msg, "MsgXiManualResponse")
	return fmt.Sprintf("%s%s", prefix, text)
}

func (x *XiPersonality) XiifyAudio(msg *tgbotapi.Message, text string) string {
	prefix := x.localization.LocalizeBy(msg, "MsgAudioSuccess")
	return fmt.Sprintf("%s%s", prefix, text)
}

// XiifyManualPlain returns text without any Xi prefix (for callback messages)
func (x *XiPersonality) XiifyManualPlain(text string) string {
	return text
}