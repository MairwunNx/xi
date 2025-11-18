package texting

import (
	"fmt"
	"ximanager/sources/localization"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type XiMessenger struct {
	localization *localization.LocalizationManager
}

func NewXiMessenger(localization *localization.LocalizationManager) *XiMessenger {
	return &XiMessenger{localization: localization}
}

func (x *XiMessenger) Xiify(msg *tgbotapi.Message, text string) string {
	prefix := x.localization.LocalizeBy(msg, "MsgXiResponse", nil)
	return fmt.Sprintf("%s%s", prefix, text)
}

func (x *XiMessenger) XiifyManual(msg *tgbotapi.Message, text string) string {
	prefix := x.localization.LocalizeBy(msg, "MsgXiManualResponse", nil)
	return fmt.Sprintf("%s%s", prefix, text)
}

func (x *XiMessenger) XiifyAudio(msg *tgbotapi.Message, text string) string {
	prefix := x.localization.LocalizeBy(msg, "MsgAudioSuccess", nil)
	return fmt.Sprintf("%s%s", prefix, text)
}