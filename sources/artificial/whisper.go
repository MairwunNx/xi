package artificial

import (
	"context"
	"os"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	"github.com/sashabaranov/go-openai"
)

type Whisper struct {
	ai           *openai.Client
	config       *AIConfig
	usageLimiter *UsageLimiter
	donations    *repository.DonationsRepository
}

func NewWhisper(
	config *AIConfig,
	ai *openai.Client,
	usageLimiter *UsageLimiter,
	donations *repository.DonationsRepository,
) *Whisper {
	return &Whisper{
		ai:           ai,
		config:       config,
		usageLimiter: usageLimiter,
		donations:    donations,
	}
}

func (w *Whisper) Whisperify(log *tracing.Logger, file *os.File, user *entities.User) (string, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	userGrade, err := w.donations.GetUserGrade(log, user)
	if err != nil {
		log.W("Failed to get user grade, using bronze as default", tracing.InnerError, err)
		userGrade = platform.GradeBronze
	}

	limitResult, err := w.usageLimiter.checkAndIncrement(log, user.UserID, userGrade, UsageTypeWhisper)
	if err != nil {
		log.E("Failed to check usage limits", tracing.InnerError, err)
		return "", err
	}

	if limitResult.Exceeded {
		if limitResult.IsDaily {
			return texting.MsgDailyLimitExceeded, nil
		}
		return texting.MsgMonthlyLimitExceeded, nil
	}

	log = log.With(tracing.AiKind, "openai/whisper", tracing.AiModel, w.config.WhisperModel)
	log.I("stt requested")

	request := openai.AudioRequest{Model: w.config.WhisperModel, FilePath: file.Name(), Language: "ru"}
	response, err := w.ai.CreateTranscription(ctx, request)
	if err != nil {
		switch e := err.(type) {
		case *openai.APIError:
			if e.Code == 402 {
				return texting.MsgInsufficientCredits, nil
			}
		}		
		log.E("Failed to transcribe audio", tracing.InnerError, err)
		return "", err
	}

	log.I("stt completed")
	return response.Text, nil
}