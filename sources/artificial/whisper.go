package artificial

import (
	"context"
	"os"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/sashabaranov/go-openai"
)

type Whisper struct {
	ai *openai.Client
	config *AIConfig
}

func NewWhisper(config *AIConfig, ai *openai.Client) *Whisper {
	return &Whisper{ai: ai, config: config}
}

func (w *Whisper) Whisperify(log *tracing.Logger, file *os.File) (string, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	log = log.With(tracing.AiKind, "openai/whisper", tracing.AiModel, w.config.WhisperModel)
	log.I("stt requested")

	request := openai.AudioRequest{Model: w.config.WhisperModel, FilePath: file.Name(), Language: "ru"}
	response, err := w.ai.CreateTranscription(ctx, request)
	if err != nil {
		log.E("Failed to transcribe audio", tracing.InnerError, err)
		return "", err
	}

	log.I("stt completed")
	return response.Text, nil
}
