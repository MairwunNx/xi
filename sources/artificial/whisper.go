package artificial

import (
	"context"
	"os"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/localization"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sashabaranov/go-openai"
)

type Whisper struct {
	ai           *openai.Client
	config       *configuration.Config
	usageLimiter *UsageLimiter
	donations    *repository.DonationsRepository
	localization *localization.LocalizationManager
}

func NewWhisper(
	config *configuration.Config,
	ai *openai.Client,
	usageLimiter *UsageLimiter,
	donations *repository.DonationsRepository,
	localization *localization.LocalizationManager,
) *Whisper {
	return &Whisper{
		ai:           ai,
		config:       config,
		usageLimiter: usageLimiter,
		donations:    donations,
		localization: localization,
	}
}

func (w *Whisper) Whisperify(log *tracing.Logger, msg *tgbotapi.Message, file *os.File, user *entities.User) (string, error) {
	defer tracing.ProfilePoint(log, "Whisper whisperify completed", "artificial.whisper.whisperify")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	userGrade, err := w.donations.GetUserGrade(log, user)
	if err != nil {
		log.W("Failed to get user grade, using bronze as default", tracing.InnerError, err)
		userGrade = platform.GradeBronze
	}

	duration := 0
	if msg.Voice != nil {
		duration = msg.Voice.Duration
	} else if msg.Audio != nil {
		duration = msg.Audio.Duration
	} else if msg.VideoNote != nil {
		duration = msg.VideoNote.Duration
	} else if msg.Video != nil {
		duration = msg.Video.Duration
	}

	if duration > 900 {
		log.W("Audio duration exceeded limit", "duration", duration, "limit", 900)
		return w.localization.LocalizeBy(msg, "MsgVoiceDurationExceeded"), nil
	}

	limitResult, err := w.usageLimiter.checkAndIncrement(log, user.UserID, userGrade, UsageTypeWhisper)
	if err != nil {
		log.E("Failed to check usage limits", tracing.InnerError, err)
		return "", err
	}

	if limitResult.Exceeded {
		if limitResult.IsDaily {
			return w.localization.LocalizeBy(msg, "MsgDailyLimitExceeded"), nil
		}
		return w.localization.LocalizeBy(msg, "MsgMonthlyLimitExceeded"), nil
	}

	log = log.With(tracing.AiKind, "openai/whisper", tracing.AiModel, w.config.AI.WhisperModel)
	log.I("stt requested")

	request := openai.AudioRequest{Model: w.config.AI.WhisperModel, FilePath: file.Name(), Language: "ru"}
	response, err := w.ai.CreateTranscription(ctx, request)
	if err != nil {
		switch e := err.(type) {
		case *openai.APIError:
			if e.Code == 402 {
				return w.localization.LocalizeBy(msg, "MsgInsufficientCredits"), nil
			}
		}
		log.E("Failed to transcribe audio", tracing.InnerError, err)
		return "", err
	}

	log.I("stt completed")
	return response.Text, nil
}
