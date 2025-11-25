package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
	"ximanager/sources/artificial"
	"ximanager/sources/features"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting/format"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// =========================  /xi command handlers  =========================

func (x *TelegramHandler) XiCommandText(log *tracing.Logger, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Xi command text completed", "telegram.command.xi.text", "chat_id", msg.Chat.ID)()
	req := x.GetRequestText(msg)
	if req == "" {
		helpMsg := x.localization.LocalizeBy(msg, "MsgHelpText")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, helpMsg))
		return
	}

	x.diplomat.StartTyping(msg.Chat.ID)
	defer x.diplomat.StopTyping(msg.Chat.ID)

	persona := msg.From.FirstName + " " + msg.From.LastName + " (@" + msg.From.UserName + ")"

	if x.features.IsEnabled(features.FeatureStreamingResponses) {
		streamReply, err := x.diplomat.StartStreamingReply(log, msg)
		if err != nil {
			log.E("Failed to start streaming reply", tracing.InnerError, err)
			x.xiCommandTextNonStreaming(log, msg, req, persona)
			return
		}

		// Status animation state
		statusAnimator := NewStatusAnimator(
			x.localization.LocalizeBy(msg, "MsgStreamingThinking"),
			x.localization.LocalizeBy(msg, "MsgStreamingSearching"),
			streamReply,
		)
		defer statusAnimator.Stop()

		var streamCallback artificial.StreamCallback = func(chunk artificial.StreamChunk) {
			// Handle status changes
			if chunk.Status != artificial.StreamStatusNone {
				statusAnimator.SetStatus(chunk.Status)
				return
			}

			// Stop animation when content arrives
			statusAnimator.Stop()

			if chunk.Error != nil {
				errorMsg := x.localization.LocalizeBy(msg, "MsgErrorResponse")
				streamReply.FinishWithError(errorMsg)
				return
			}
			if chunk.Done {
				finalText := x.personality.Xiify(msg, chunk.Content)
				if strings.TrimSpace(chunk.Content) == "" {
					streamReply.FinishWithError(x.localization.LocalizeBy(msg, "MsgErrorResponse"))
				} else {
					streamReply.Finish(finalText)
				}
				return
			}
			// Update with partial content (no Xiify during streaming for performance)
			streamReply.Update(chunk.Content)
		}

		_, err = x.dialer.Dial(log, msg, req, "", persona, true, streamCallback)
		if err != nil {
			log.E("Error from dialer in streaming mode", tracing.InnerError, err)
		}
	} else {
		x.xiCommandTextNonStreaming(log, msg, req, persona)
	}
}

func (x *TelegramHandler) xiCommandTextNonStreaming(log *tracing.Logger, msg *tgbotapi.Message, req string, persona string) {
	response, err := x.dialer.Dial(log, msg, req, "", persona, true, nil)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgErrorResponse")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	if strings.TrimSpace(response) == "" {
		log.W("Empty response from AI orchestrator", "response", response)
		errorMsg := x.localization.LocalizeBy(msg, "MsgErrorResponse")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	x.diplomat.Reply(log, msg, x.personality.Xiify(msg, response))
}

func (x *TelegramHandler) XiCommandPhoto(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Xi command photo completed", "telegram.command.xi.photo", "chat_id", msg.Chat.ID)()
	x.diplomat.StartTyping(msg.Chat.ID)
	defer x.diplomat.StopTyping(msg.Chat.ID)

	photo := msg.Photo[len(msg.Photo)-1]

	fileConfig := tgbotapi.FileConfig{FileID: photo.FileID}
	file, err := x.diplomat.bot.GetFile(fileConfig)
	if err != nil {
		log.E("Error getting file", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgErrorResponse"))
		return
	}

	iurl := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", x.diplomat.bot.Token, file.FilePath)

	req := ""

	if msg.IsCommand() {
		req = msg.CommandArguments()
	}

	if req == "" {
		req = msg.Caption
	}

	persona := msg.From.FirstName + " " + msg.From.LastName + " (@" + msg.From.UserName + ")"

	if x.features.IsEnabled(features.FeatureStreamingResponses) {
		streamReply, err := x.diplomat.StartStreamingReply(log, msg)
		if err != nil {
			log.E("Failed to start streaming reply for photo", tracing.InnerError, err)
			x.xiCommandPhotoNonStreaming(log, msg, req, iurl, persona)
			return
		}

		statusAnimator := NewStatusAnimator(
			x.localization.LocalizeBy(msg, "MsgStreamingThinking"),
			x.localization.LocalizeBy(msg, "MsgStreamingSearching"),
			streamReply,
		)
		defer statusAnimator.Stop()

		var streamCallback artificial.StreamCallback = func(chunk artificial.StreamChunk) {
			if chunk.Status != artificial.StreamStatusNone {
				statusAnimator.SetStatus(chunk.Status)
				return
			}

			statusAnimator.Stop()

			if chunk.Error != nil {
				errorMsg := x.localization.LocalizeBy(msg, "MsgErrorResponse")
				streamReply.FinishWithError(errorMsg)
				return
			}
			if chunk.Done {
				finalText := x.personality.Xiify(msg, chunk.Content)
				if strings.TrimSpace(chunk.Content) == "" {
					streamReply.FinishWithError(x.localization.LocalizeBy(msg, "MsgErrorResponse"))
				} else {
					streamReply.Finish(finalText)
				}
				return
			}
			streamReply.Update(chunk.Content)
		}

		_, err = x.dialer.Dial(log, msg, req, iurl, persona, true, streamCallback)
		if err != nil {
			log.E("Error from dialer in streaming mode for photo", tracing.InnerError, err)
		}
	} else {
		x.xiCommandPhotoNonStreaming(log, msg, req, iurl, persona)
	}
}

func (x *TelegramHandler) xiCommandPhotoNonStreaming(log *tracing.Logger, msg *tgbotapi.Message, req, iurl, persona string) {
	response, err := x.dialer.Dial(log, msg, req, iurl, persona, true, nil)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgErrorResponse"))
		return
	}

	x.diplomat.Reply(log, msg, x.personality.Xiify(msg, response))
}

func (x *TelegramHandler) XiCommandAudio(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, replyMsg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Xi command audio completed", "telegram.command.xi.audio", "chat_id", msg.Chat.ID)()
	x.diplomat.StartTyping(msg.Chat.ID)
	defer x.diplomat.StopTyping(msg.Chat.ID)

	var fileID string
	var fileExt string

	if replyMsg.Voice != nil {
		fileID = replyMsg.Voice.FileID
		fileExt = ".ogg"
	} else if replyMsg.VideoNote != nil {
		fileID = replyMsg.VideoNote.FileID
		fileExt = ".mp4"
	} else if replyMsg.Audio != nil {
		fileID = replyMsg.Audio.FileID
		fileExt = ".mp3"
	} else if replyMsg.Video != nil {
		fileID = replyMsg.Video.FileID
		fileExt = ".mp4"
	} else {
		log.W("Unsupported audio/video file type")
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgAudioUnsupported"))
		return
	}

	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	file, err := x.diplomat.bot.GetFile(fileConfig)
	if err != nil {
		log.E("Error getting file", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgAudioError"))
		return
	}

	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", x.diplomat.bot.Token, file.FilePath)

	tempFile, err := x.downloadAudioFile(log, fileURL, fileExt)
	if err != nil {
		log.E("Error downloading audio file", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgAudioError"))
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	userPrompt := strings.TrimSpace(msg.CommandArguments())
	transcriptedText, err := x.whisper.Whisperify(log, msg, tempFile, user)
	if err != nil {
		log.E("Error transcribing audio", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgAudioError"))
		return
	}

	if userPrompt != "" {
		persona := msg.From.FirstName + " " + msg.From.LastName + " (@" + msg.From.UserName + ")"
		response, err := x.dialer.Dial(log, msg, transcriptedText, "", persona, false, nil)
		if err != nil {
			log.E("Error processing with lightweight model", tracing.InnerError, err)
			x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgAudioError"))
			return
		}
		x.diplomat.Reply(log, msg, x.personality.XiifyAudio(msg, response))
	} else {
		x.diplomat.ReplyAudio(log, msg, x.personality.XiifyAudio(msg, transcriptedText))
	}
}

func (x *TelegramHandler) downloadAudioFile(log *tracing.Logger, fileURL string, fileExt string) (*os.File, error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	tempFile, err := os.CreateTemp("", "audio_*"+fileExt)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return nil, err
	}

	tempFile.Seek(0, 0)

	log.I("Audio file downloaded", "file_path", tempFile.Name(), "file_size", resp.ContentLength)
	return tempFile, nil
}

// =========================  /modes command handlers  =========================

func (x *TelegramHandler) ModeCommandSwitch(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	mode, err := x.modes.SwitchMode(log, msg.Chat.ID, msg.From.ID)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorSwitching")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeChanged", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) ModeCommandAdd(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, chatID int64, modeType string, modeName string, configJSON string) {
	var config repository.ModeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorConfigParse")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	if strings.TrimSpace(config.Prompt) == "" {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorConfigPrompt")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	mode, err := x.modes.AddModeWithConfig(log, chatID, modeType, modeName, &config, msg.From.ID)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorAdd")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeAdded", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) ModeCommandList(log *tracing.Logger, msg *tgbotapi.Message, chatID int64) {
	user, err := x.users.GetUserByEid(log, msg.From.ID)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorGettingList")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	modes, err := x.modes.GetModesByChat(log, chatID, user)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorGettingList")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	if len(modes) == 0 {
		noModesMsg := x.localization.LocalizeBy(msg, "MsgModeNoModesAvailable")
		x.diplomat.Reply(log, msg, noModesMsg)
		return
	}

	currentMode, err := x.modes.GetModeByChat(log, chatID)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorGettingList")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	message := x.localization.LocalizeBy(msg, "MsgModeListHeader")
	for _, mode := range modes {
		modeData := map[string]interface{}{
			"Name": mode.Name,
			"Type": mode.Type,
		}

		if mode.ID == currentMode.ID {
			message += x.localization.LocalizeByTd(msg, "MsgModeListItemCurrent", modeData)
		} else {
			message += x.localization.LocalizeByTd(msg, "MsgModeListItem", modeData)
		}
	}

	x.diplomat.Reply(log, msg, message)
}

func (x *TelegramHandler) ModeCommandDisable(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	mode.IsEnabled = platform.BoolPtr(false)
	_, err := x.modes.UpdateMode(log, mode)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorDisable")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeDisabled", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) ModeCommandEnable(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	mode.IsEnabled = platform.BoolPtr(true)
	_, err := x.modes.UpdateMode(log, mode)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorEnable")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeEnabled", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) ModeCommandDelete(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	err := x.modes.DeleteMode(log, mode)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorDelete")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeDeleted", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) ModeCommandEdit(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string, configJSON string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	var config repository.ModeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorConfigParse")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	if strings.TrimSpace(config.Prompt) == "" {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorConfigPrompt")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	err := x.modes.UpdateModeConfig(log, mode.ID, &config)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorEdit")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeEdited", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) retrieveModeByChat(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) *entities.Mode {
	mode, err := x.modes.GetModeByChat(log, chatID)
	if err != nil {
		errorMsg := x.localization.LocalizeByTd(msg, "MsgModeNotFoundGeneral", map[string]interface{}{
			"Mode": modeType,
		})
		x.diplomat.Reply(log, msg, errorMsg)
		return nil
	}
	return mode
}

// =========================  /users command handlers  =========================

func (x *TelegramHandler) UsersCommandRemove(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	err := x.users.DeleteUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersErrorRemove"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersRemoved"))
}

func (x *TelegramHandler) UsersCommandEdit(log *tracing.Logger, msg *tgbotapi.Message, username string, inputRights []string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	rights := x.treat(inputRights)
	if len(rights) == 0 {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersInvalidRights"))
		return
	}

	user.Rights = rights
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgUsersErrorEdit")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgUsersEdited", map[string]interface{}{
		"Username": *user.Username,
		"Rights":   strings.Join(rights, ", "),
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) UsersCommandDisable(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsActive = platform.BoolPtr(false)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgUsersErrorDisable")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgUsersDisabled", map[string]interface{}{
		"Username": *user.Username,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) UsersCommandEnable(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsActive = platform.BoolPtr(true)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgUsersErrorEnable")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgUsersEnabled", map[string]interface{}{
		"Username": *user.Username,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) UsersCommandWindow(log *tracing.Logger, msg *tgbotapi.Message, username string, limit int64) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.WindowLimit = limit
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgUsersErrorWindow")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgUsersWindowSet", map[string]interface{}{
		"Username": *user.Username,
		"Window":   limit,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) UsersCommandStack(log *tracing.Logger, msg *tgbotapi.Message, username string, enabled bool) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsStackAllowed = platform.BoolPtr(enabled)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgUsersErrorStack")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	status := "теперь доступен"
	if !enabled {
		status = "теперь недоступен"
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgUsersStackSet", map[string]interface{}{
		"Username": *user.Username,
		"Status":   status,
	})
	x.diplomat.Reply(log, msg, successMsg)
}

func (x *TelegramHandler) retrieveUserByName(log *tracing.Logger, msg *tgbotapi.Message, username string) *entities.User {
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUserNotSpecified"))
		return nil
	}

	user, err := x.users.GetUserByName(log, username)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgUserNotFound", map[string]interface{}{
			"Username": username,
		}))
		return nil
	}

	return user
}

func (x *TelegramHandler) treat(rights []string) []string {
	var treated []string
	for _, right := range rights {
		right = strings.ToLower(strings.TrimSpace(right))
		if slices.Contains(repository.AvailableRights, right) {
			treated = append(treated, right)
		}
	}
	return treated
}

// =========================  /stats command handlers  =========================

func (x *TelegramHandler) StatsCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Stats command completed", "telegram.command.stats", "chat_id", msg.Chat.ID)()
	statsErrorMsg := x.localization.LocalizeBy(msg, "MsgStatsError")

	totalQuestions, err := x.messages.GetTotalUserQuestionsCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	chatQuestions, err := x.messages.GetUserQuestionsInChatCount(log, msg.Chat.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userQuestions, err := x.messages.GetUserPersonalQuestionsCount(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userChatQuestions, err := x.messages.GetUserPersonalQuestionsInChatCount(log, user, msg.Chat.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	totalUsers, err := x.users.GetTotalUsersCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	totalChats, err := x.messages.GetUniqueChatCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	totalCost, err := x.usage.GetTotalCost(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	totalCostLastMonth, err := x.usage.GetTotalCostLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	avgDailyCost, err := x.usage.GetAverageDailyCost(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	totalTokens, err := x.usage.GetTotalTokens(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	totalTokensLastMonth, err := x.usage.GetTotalTokensLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	anotherTotalCost, err := x.usage.GetTotalAnotherCost(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	anotherTotalCostLastMonth, err := x.usage.GetTotalAnotherCostLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	anotherTotalTokens, err := x.usage.GetTotalAnotherTokens(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	anotherTotalTokensLastMonth, err := x.usage.GetTotalAnotherTokensLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userCost, err := x.usage.GetUserCost(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userCostLastMonth, err := x.usage.GetUserCostLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userAvgDailyCost, err := x.usage.GetUserAverageDailyCost(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userTokens, err := x.usage.GetUserTokens(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userTokensLastMonth, err := x.usage.GetUserTokensLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userAnotherCost, err := x.usage.GetUserAnotherCost(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userAnotherCostLastMonth, err := x.usage.GetUserAnotherCostLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userAnotherTokens, err := x.usage.GetUserAnotherTokens(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	userAnotherTokensLastMonth, err := x.usage.GetUserAnotherTokensLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, statsErrorMsg))
		return
	}

	title := x.localization.LocalizeBy(msg, "MsgStatsTitle")

	generalStats := x.localization.LocalizeByTd(msg, "MsgStatsGeneral", map[string]interface{}{
		"TotalQuestions":           format.Numberify(totalQuestions),
		"ChatQuestions":            format.Numberify(chatQuestions),
		"TotalCost":                totalCost.String(),
		"MonthlyCost":              totalCostLastMonth.String(),
		"DailyCost":                format.CurrencifyDecimal(avgDailyCost),
		"TotalTokens":              format.Numberify(totalTokens),
		"MonthlyTokens":            format.Numberify(totalTokensLastMonth),
		"AnotherTotalCost":         anotherTotalCost.String(),
		"AnotherMonthlyCost":       anotherTotalCostLastMonth.String(),
		"AnotherTotalTokens":       format.Numberify(anotherTotalTokens),
		"AnotherMonthlyTokens":     format.Numberify(anotherTotalTokensLastMonth),
	})

	personalStats := x.localization.LocalizeByTd(msg, "MsgStatsPersonal", map[string]interface{}{
		"TotalQuestions":           format.Numberify(userQuestions),
		"ChatQuestions":            format.Numberify(userChatQuestions),
		"TotalCost":                format.CurrencifyDecimal(userCost),
		"MonthlyCost":              format.CurrencifyDecimal(userCostLastMonth),
		"DailyCost":                format.CurrencifyDecimal(userAvgDailyCost),
		"TotalTokens":              format.Numberify(userTokens),
		"MonthlyTokens":            format.Numberify(userTokensLastMonth),
		"AnotherTotalCost":         format.CurrencifyDecimal(userAnotherCost),
		"AnotherMonthlyCost":       format.CurrencifyDecimal(userAnotherCostLastMonth),
		"AnotherTotalTokens":       format.Numberify(userAnotherTokens),
		"AnotherMonthlyTokens":     format.Numberify(userAnotherTokensLastMonth),
	})

	usersStats := x.localization.LocalizeByTd(msg, "MsgStatsUsers", map[string]interface{}{
		"Users": format.Numberify(totalUsers),
		"Chats": format.Numberify(totalChats),
	})

	response := title + generalStats + personalStats + usersStats
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, response))
}

func (x *TelegramHandler) PersonalizationCommandSet(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, prompt string) {
	promptRunes := []rune(prompt)
	if len(promptRunes) < 12 {
		errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationTooShort")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	if len(promptRunes) > 1024 {
		errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationTooLong")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	validation, err := x.agents.ValidatePersonalization(log, prompt)
	if err != nil {
		log.E("Failed to validate personalization with agent", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationValidationError")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	if validation.Confidence <= 0.68 {
		log.I("Personalization validation failed", "confidence", validation.Confidence)
		errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationValidationFailed")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	_, err = x.personalizations.CreateOrUpdatePersonalization(log, user, prompt)
	if err != nil {
		if errors.Is(err, repository.ErrPersonalizationTooShort) {
			errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationTooShort")
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
			return
		}

		if errors.Is(err, repository.ErrPersonalizationTooLong) {
			errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationTooLong")
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
			return
		}

		log.E("Failed to create personalization", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationErrorAdd")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	log.I("Personalization set successfully", "confidence", validation.Confidence)
	successMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationAdded")
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) PersonalizationCommandRemove(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	err := x.personalizations.DeletePersonalization(log, user)
	if err != nil {
		if errors.Is(err, repository.ErrPersonalizationNotFound) {
			errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationNotFound")
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
			return
		}

		log.E("Failed to delete personalization", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationErrorRemove")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	successMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationRemoved")
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) PersonalizationCommandPrint(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	personalization, err := x.personalizations.GetPersonalizationByUser(log, user)
	if err != nil {
		if errors.Is(err, repository.ErrPersonalizationNotFound) {
			errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationNotFound")
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
			return
		}

		log.E("Failed to get personalization", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationErrorPrint")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	response := x.localization.LocalizeByTd(msg, "MsgPersonalizationPrint", map[string]interface{}{
		"Info": personalization.Prompt,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, response))
}

// =========================  /context command handlers  =========================

func (x *TelegramHandler) ContextCommandRefresh(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	err := x.contextManager.Clear(log, platform.ChatID(msg.Chat.ID))
	if err != nil {
		log.E("Failed to clear context", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgContextRefreshError")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	successMsg := x.localization.LocalizeBy(msg, "MsgContextRefreshed")
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) ContextCommandEnable(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	err := x.contextManager.SetEnabled(log, platform.ChatID(msg.Chat.ID), true)
	if err != nil {
		log.E("Failed to enable context", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgContextEnableError")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	successMsg := x.localization.LocalizeBy(msg, "MsgContextEnabled")
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) ContextCommandDisable(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	err := x.contextManager.SetEnabled(log, platform.ChatID(msg.Chat.ID), false)
	if err != nil {
		log.E("Failed to disable context", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgContextDisableError")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	successMsg := x.localization.LocalizeBy(msg, "MsgContextDisabled")
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) ContextCommandInfo(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	grade, err := x.donations.GetUserGrade(log, user)
	if err != nil {
		log.W("Failed to get user grade, using bronze", tracing.InnerError, err)
		grade = platform.GradeBronze
	}

	stats, err := x.contextManager.GetStats(log, platform.ChatID(msg.Chat.ID), grade)
	if err != nil {
		log.E("Failed to get context stats", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgContextInfoError")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	statusText := x.localization.LocalizeBy(msg, "MsgContextStatusEnabled")
	if !stats.Enabled {
		statusText = x.localization.LocalizeBy(msg, "MsgContextStatusDisabled")
	}

	infoMsg := x.localization.LocalizeByTd(msg, "MsgContextInfo", map[string]interface{}{
		"Status":          statusText,
		"CurrentMessages": format.Numberify(int64(stats.CurrentMessages)),
		"MaxMessages":     format.Numberify(int64(stats.MaxMessages)),
		"CurrentTokens":   format.Numberify(int64(stats.CurrentTokens)),
		"MaxTokens":       format.Numberify(int64(stats.MaxTokens)),
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, infoMsg))
}

// =========================  /ban and /pardon command handlers  =========================

func (x *TelegramHandler) BanCommandApply(log *tracing.Logger, msg *tgbotapi.Message, username string, reason string, duration string) {
	targetUser := x.retrieveUserByName(log, msg, username)
	if targetUser == nil {
		return
	}

	_, err := x.bans.ParseDuration(duration)
	if err != nil {
		var errorMsg string
		if errors.Is(err, repository.ErrDurationTooLong) {
			errorMsg = x.localization.LocalizeBy(msg, "MsgBanErrorTooLong")
		} else {
			errorMsg = x.localization.LocalizeBy(msg, "MsgBanErrorInvalid")
		}
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	_, err = x.bans.CreateBan(log, targetUser.ID, msg.Chat.ID, reason, duration)
	if err != nil {
		log.E("Failed to create ban", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgBanErrorCreate")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	displayName := *targetUser.Username
	banMsg := x.localization.LocalizeByTd(msg, "MsgBanApplied", map[string]interface{}{
		"Username": displayName,
		"Duration": duration,
		"Reason":   reason,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, banMsg))
}

func (x *TelegramHandler) PardonCommandApply(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	targetUser := x.retrieveUserByName(log, msg, username)
	if targetUser == nil {
		return
	}

	_, err := x.bans.GetActiveBan(log, targetUser.ID)
	if err != nil {
		displayName := *targetUser.Username
		errorMsg := x.localization.LocalizeByTd(msg, "MsgBanErrorNotFound", map[string]interface{}{
			"Username": displayName,
		})
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	err = x.bans.DeleteBansByUser(log, targetUser.ID)
	if err != nil {
		log.E("Failed to pardon user", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgBanErrorRemove")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	displayName := *targetUser.Username
	successMsg := x.localization.LocalizeByTd(msg, "MsgBanPardon", map[string]interface{}{
		"Username": displayName,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

// =========================  /health command handlers  =========================

func (x *TelegramHandler) HealthCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	dbStatus := x.localization.LocalizeBy(msg, "MsgHealthStatusOk")
	if err := x.health.CheckDatabaseHealth(log); err != nil {
		dbStatus = x.localization.LocalizeBy(msg, "MsgHealthStatusFail")
	}

	redisStatus := x.localization.LocalizeBy(msg, "MsgHealthStatusOk")
	if err := x.health.CheckRedisHealth(log); err != nil {
		redisStatus = x.localization.LocalizeBy(msg, "MsgHealthStatusFail")
	}

	systemStatus := x.localization.LocalizeBy(msg, "MsgHealthStatusOk")

	uptime := time.Since(platform.GetAppStartTime()).Truncate(time.Second)
	version := platform.GetAppVersion()
	buildTime := platform.GetAppBuildTime()

	title := x.localization.LocalizeBy(msg, "MsgHealthTitle")

	dbMsg := x.localization.LocalizeByTd(msg, "MsgHealthDatabase", map[string]interface{}{
		"Status": dbStatus,
	})

	redisMsg := x.localization.LocalizeByTd(msg, "MsgHealthRedis", map[string]interface{}{
		"Status": redisStatus,
	})

	systemMsg := x.localization.LocalizeByTd(msg, "MsgHealthSystem", map[string]interface{}{
		"Status": systemStatus,
	})

	uptimeMsg := x.localization.LocalizeByTd(msg, "MsgHealthUptime", map[string]interface{}{
		"Uptime": uptime,
	})

	versionMsg := x.localization.LocalizeByTd(msg, "MsgHealthVersion", map[string]interface{}{
		"Version": version,
	})

	buildTimeMsg := x.localization.LocalizeByTd(msg, "MsgHealthBuildTime", map[string]interface{}{
		"BuildTime": buildTime,
	})

	response := title + dbMsg + redisMsg + systemMsg + uptimeMsg + versionMsg + buildTimeMsg
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, response))
}

func (x *TelegramHandler) ThisCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	grade, err := x.donations.GetUserGrade(log, user)
	if err != nil {
		log.W("Failed to get user grade, using bronze", tracing.InnerError, err)
		grade = platform.GradeBronze
	}

	gradeEmoji := getGradeEmoji(grade)
	gradeName := getGradeNameRu(grade)
	accountAge := x.dateTimeFormatter.Ageify(msg, user.CreatedAt)

	infoData := map[string]interface{}{
		"Emoji":       gradeEmoji,
		"Grade":       gradeName,
		"AccountDate": accountAge,
		"TelegramID":  user.UserID,
		"Name":        *user.Fullname,
		"Username":    *user.Username,
		"InternalID":  user.ID,
		"Rights":      user.Rights,
		"ChatID":      msg.Chat.ID,
		"ChatType":    msg.Chat.Type,
		"ChatTitle":   msg.Chat.Title,
	}

	response := x.localization.LocalizeByTd(msg, "MsgThisInfo", infoData)
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, response))
}

func getGradeEmoji(grade platform.UserGrade) string {
	switch grade {
	case platform.GradeGold:
		return "💎"
	case platform.GradeSilver:
		return "🥈"
	case platform.GradeBronze:
		return "🥉"
	default:
		return "❓"
	}
}

func getGradeNameRu(grade platform.UserGrade) string {
	switch grade {
	case platform.GradeGold:
		return "Золотой"
	case platform.GradeSilver:
		return "Серебряный"
	case platform.GradeBronze:
		return "Бронзовый"
	default:
		return "Неизвестный"
	}
}

// =========================  /broadcast command handlers  =========================

// =========================  /tariff command handlers  =========================

func (x *TelegramHandler) TariffCommandAdd(log *tracing.Logger, msg *tgbotapi.Message, key string, configJSON string) {
	defer tracing.ProfilePoint(log, "Tariff command add completed", "telegram.command.tariff.add", "key", key)()

	var config repository.TariffConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgTariffErrorConfigParse")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	tariff, err := x.tariffs.CreateTariff(log, key, &config)
	if err != nil {
		log.E("Failed to create tariff", tracing.InnerError, err)

		var errorMsg string
		switch {
		case errors.Is(err, repository.ErrTariffKeyEmpty):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorKeyEmpty")
		case errors.Is(err, repository.ErrTariffKeyTooLong):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorKeyTooLong")
		case errors.Is(err, repository.ErrTariffNameEmpty):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorNameEmpty")
		case errors.Is(err, repository.ErrTariffNameTooLong):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorNameTooLong")
		case errors.Is(err, repository.ErrTariffInvalidEffort):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorInvalidEffort")
		case errors.Is(err, repository.ErrTariffInvalidLimit):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorInvalidLimit")
		case errors.Is(err, repository.ErrTariffModelsEmpty):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorModelsEmpty")
		case errors.Is(err, repository.ErrTariffModelNameEmpty):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorModelNameEmpty")
		default:
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorCreate")
		}
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgTariffAdded", map[string]interface{}{
		"Key":         tariff.Key,
		"DisplayName": tariff.DisplayName,
		"ID":          tariff.ID,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) TariffCommandList(log *tracing.Logger, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Tariff command list completed", "telegram.command.tariff.list")()

	tariffs, err := x.tariffs.GetAllLatest(log)
	if err != nil {
		log.E("Failed to get tariffs list", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgTariffErrorList")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	if len(tariffs) == 0 {
		noTariffsMsg := x.localization.LocalizeBy(msg, "MsgTariffNoTariffs")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noTariffsMsg))
		return
	}

	var builder strings.Builder
	header := x.localization.LocalizeBy(msg, "MsgTariffListHeader")
	builder.WriteString(header)

	for _, t := range tariffs {
		itemMsg := x.localization.LocalizeByTd(msg, "MsgTariffListItem", map[string]interface{}{
			"Key":         t.Key,
			"DisplayName": t.DisplayName,
			"ID":          t.ID,
		})
		builder.WriteString(itemMsg)
	}

	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, builder.String()))
}

func (x *TelegramHandler) TariffCommandGet(log *tracing.Logger, msg *tgbotapi.Message, key string) {
	defer tracing.ProfilePoint(log, "Tariff command get completed", "telegram.command.tariff.get", "key", key)()

	tariff, err := x.tariffs.GetLatestByKey(log, key)
	if err != nil {
		log.E("Failed to get tariff", tracing.InnerError, err, "key", key)
		errorMsg := x.localization.LocalizeByTd(msg, "MsgTariffNotFound", map[string]interface{}{
			"Key": key,
		})
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	modelsFormatted := x.formatTariffModels(log, msg, tariff.DialerModels)

	infoMsg := x.localization.LocalizeByTd(msg, "MsgTariffInfo", map[string]interface{}{
		"ID":                    tariff.ID,
		"Key":                   tariff.Key,
		"DisplayName":           tariff.DisplayName,
		"Models":                modelsFormatted,
		"DialerReasoningEffort": tariff.DialerReasoningEffort,
		"ContextTTLSeconds":     tariff.ContextTTLSeconds,
		"ContextMaxMessages":    tariff.ContextMaxMessages,
		"ContextMaxTokens":      tariff.ContextMaxTokens,
		"UsageVisionDaily":      tariff.UsageVisionDaily,
		"UsageVisionMonthly":    tariff.UsageVisionMonthly,
		"UsageDialerDaily":      tariff.UsageDialerDaily,
		"UsageDialerMonthly":    tariff.UsageDialerMonthly,
		"UsageWhisperDaily":     tariff.UsageWhisperDaily,
		"UsageWhisperMonthly":   tariff.UsageWhisperMonthly,
		"SpendingDailyLimit":    tariff.SpendingDailyLimit.String(),
		"SpendingMonthlyLimit":  tariff.SpendingMonthlyLimit.String(),
		"CreatedAt":             tariff.CreatedAt.Format("02.01.2006 15:04:05"),
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, infoMsg))
}

func (x *TelegramHandler) formatTariffModels(log *tracing.Logger, msg *tgbotapi.Message, modelsJSON []byte) string {
	var models []repository.ModelMeta
	if err := json.Unmarshal(modelsJSON, &models); err != nil {
		log.W("Failed to parse tariff models for display", tracing.InnerError, err)
		return x.localization.LocalizeBy(msg, "MsgTariffModelsParseError")
	}

	if len(models) == 0 {
		return x.localization.LocalizeBy(msg, "MsgTariffNoModels")
	}

	var builder strings.Builder
	for i, model := range models {
		itemMsg := x.localization.LocalizeByTd(msg, "MsgTariffModelItem", map[string]interface{}{
			"Name":      model.Name,
			"AAI":       model.AAI,
			"InputCost": model.InputPricePerM,
			"OutputCost": model.OutputPricePerM,
			"Context":   model.CtxTokens,
		})
		builder.WriteString(itemMsg)
		if i < len(models)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func (x *TelegramHandler) BroadcastCommandApply(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, text string) {
	defer tracing.ProfilePoint(log, "Broadcast command apply completed", "telegram.command.broadcast.apply", "user_id", user.ID)()

	// 1. Save broadcast to DB
	_, err := x.broadcast.CreateBroadcast(log, user.ID, text)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgBroadcastErrorCreate")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	// 2. Get all chat IDs
	chatIDs, err := x.messages.GetAllChatIDs(log)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgBroadcastErrorGetChats")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	// 3. Send broadcast
	successCount := 0
	failCount := 0

	x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgBroadcastStarted"))

	for _, chatID := range chatIDs {
		// Skip the sender's chat
		if chatID == msg.Chat.ID {
			continue
		}

		unsubscribeText := x.localization.LocalizeBy(msg, "MsgBroadcastUnsubscribe")
		err := x.diplomat.SendBroadcastMessage(log, chatID, text, unsubscribeText)
		if err != nil {
			failCount++
			log.W("Failed to send broadcast message", "chat_id", chatID, "error", err)
		} else {
			successCount++
		}
		
		// Add a small delay to avoid hitting Telegram limits too hard
		time.Sleep(100 * time.Millisecond)
	}

	// 4. Report result
	resultMsg := x.localization.LocalizeByTd(msg, "MsgBroadcastFinished", map[string]interface{}{
		"Success": successCount,
		"Fail":    failCount,
		"Total":   len(chatIDs),
	})
	x.diplomat.Reply(log, msg, resultMsg)
}