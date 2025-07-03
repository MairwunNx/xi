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
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// =========================  /xi command handlers  =========================

func (x *TelegramHandler) XiCommandText(log *tracing.Logger, msg *tgbotapi.Message) {
	req := x.GetRequestText(msg)
	if req == "" {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgHelpText))
		return
	}

	x.diplomat.SendTyping(log, msg.Chat.ID)

	response, err := x.dialer.Dial(log, msg, req, msg.From.FirstName+" "+msg.From.LastName, true)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgErrorResponse)
		return
	}

	if strings.TrimSpace(response) == "" {
		log.W("Empty response from AI orchestrator", "response", response)
		x.diplomat.Reply(log, msg, texting.MsgErrorResponse)
		return
	}

	x.diplomat.Reply(log, msg, texting.Xiify(response))
}

func (x *TelegramHandler) XiCommandPhoto(log *tracing.Logger, msg *tgbotapi.Message) {
	x.diplomat.SendTyping(log, msg.Chat.ID)
	
	photo := msg.Photo[len(msg.Photo)-1]

	fileConfig := tgbotapi.FileConfig{FileID: photo.FileID}
	file, err := x.diplomat.bot.GetFile(fileConfig)
	if err != nil {
		log.E("Error getting file", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, texting.MsgErrorResponse)
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

	persona := msg.From.FirstName + " " + msg.From.LastName
	user := x.users.MustGetUserByEid(log, msg.From.ID)
	response, err := x.vision.Visionify(log, iurl, user.ID, msg.Chat.ID, req, persona)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgErrorResponse)
		return
	}

	x.diplomat.Reply(log, msg, texting.Xiify(response))
}

func (x *TelegramHandler) XiCommandAudio(log *tracing.Logger, msg *tgbotapi.Message, replyMsg *tgbotapi.Message) {
	x.diplomat.SendTyping(log, msg.Chat.ID)
	
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
		x.diplomat.Reply(log, msg, texting.MsgAudioUnsupported)
		return
	}

	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	file, err := x.diplomat.bot.GetFile(fileConfig)
	if err != nil {
		log.E("Error getting file", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, texting.MsgAudioError)
		return
	}

	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", x.diplomat.bot.Token, file.FilePath)

	tempFile, err := x.downloadAudioFile(log, fileURL, fileExt)
	if err != nil {
		log.E("Error downloading audio file", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, texting.MsgAudioError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	userPrompt := strings.TrimSpace(msg.CommandArguments())
	transcriptedText, err := x.whisper.Whisperify(log, tempFile)
	if err != nil {
		log.E("Error transcribing audio", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, texting.MsgAudioError)
		return
	}

	if userPrompt != "" {
		response, err := x.dialer.Dial(log, msg, transcriptedText, msg.From.FirstName+" "+msg.From.LastName, false)
		if err != nil {
			log.E("Error processing with lightweight model", tracing.InnerError, err)
			x.diplomat.Reply(log, msg, texting.MsgAudioError)
			return
		}
		x.diplomat.Reply(log, msg, texting.XiifyAudio(response))
	} else {
		x.diplomat.Reply(log, msg, texting.XiifyAudio(transcriptedText))
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
		x.diplomat.Reply(log, msg, texting.MsgModeErrorSwitching)
		return
	}

	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgModeChanged, mode.Name, mode.Type))
}

func (x *TelegramHandler) ModeCommandAdd(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, chatID int64, modeType string, modeName string, configJSON string) {
	var config repository.ModeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeErrorConfigParse))
		return
	}

	if strings.TrimSpace(config.Prompt) == "" {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeErrorConfigPrompt))
		return
	}

	mode, err := x.modes.AddModeWithConfig(log, chatID, modeType, modeName, &config, msg.From.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgModeErrorAdd)
		return
	}
	
	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgModeAdded, mode.Name, mode.Type))
}

func (x *TelegramHandler) ModeCommandList(log *tracing.Logger, msg *tgbotapi.Message, chatID int64) {
	modes, err := x.modes.GetModesByChat(log, chatID)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgModeErrorGettingList)
		return
	}
	
	if len(modes) == 0 {
		x.diplomat.Reply(log, msg, texting.MsgModeNoModesAvailable)
		return
	}

	currentMode, err := x.modes.GetModeByChat(log, chatID)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgModeErrorGettingList)
		return
	}

	message := texting.MsgModeListHeader
	for _, mode := range modes {
		if mode.ID == currentMode.ID {
			message += fmt.Sprintf(texting.MsgModeListItemCurrent, mode.Name, mode.Type)
		} else {
			message += fmt.Sprintf(texting.MsgModeListItem, mode.Name, mode.Type)
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
		x.diplomat.Reply(log, msg, texting.MsgModeErrorDisable)
		return
	}
	
	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgModeDisabled, mode.Name, mode.Type))
}

func (x *TelegramHandler) ModeCommandEnable(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}
	
	mode.IsEnabled = platform.BoolPtr(true)
	_, err := x.modes.UpdateMode(log, mode)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgModeErrorEnable)
		return
	}
	
	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgModeEnabled, mode.Name, mode.Type))
}

func (x *TelegramHandler) ModeCommandDelete(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	err := x.modes.DeleteMode(log, mode)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgModeErrorDelete)
		return
	}

	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgModeDeleted, mode.Name, mode.Type))
}

func (x *TelegramHandler) ModeCommandEdit(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string, configJSON string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	var config repository.ModeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeErrorConfigParse))
		return
	}

	if strings.TrimSpace(config.Prompt) == "" {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgModeErrorConfigPrompt))
		return
	}

	err := x.modes.UpdateModeConfig(log, mode.ID, &config)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgModeErrorEdit)
		return
	}
	
	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgModeEdited, mode.Name, mode.Type))
}

func (x *TelegramHandler) retrieveModeByChat(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) *entities.Mode {
	mode, err := x.modes.GetModeByChat(log, chatID)
	if err != nil {
		x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgModeNotFoundGeneral, modeType))
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
		x.diplomat.Reply(log, msg, texting.MsgUsersErrorRemove)
		return
	}

	x.diplomat.Reply(log, msg, texting.MsgUsersRemoved)
}

func (x *TelegramHandler) UsersCommandEdit(log *tracing.Logger, msg *tgbotapi.Message, username string, inputRights []string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	rights := x.treat(inputRights)
	if len(rights) == 0 {
		x.diplomat.Reply(log, msg, texting.MsgUsersInvalidRights)
		return
	}

	user.Rights = rights
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgUsersErrorEdit)
		return
	}

	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgUsersEdited, *user.Username, strings.Join(rights, ", ")))
}

func (x *TelegramHandler) UsersCommandDisable(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsActive = platform.BoolPtr(false)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgUsersErrorDisable)
		return
	}

	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgUsersDisabled, *user.Username))
}

func (x *TelegramHandler) UsersCommandEnable(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsActive = platform.BoolPtr(true)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgUsersErrorEnable)
		return
	}

	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgUsersEnabled, *user.Username))
}

func (x *TelegramHandler) UsersCommandWindow(log *tracing.Logger, msg *tgbotapi.Message, username string, limit int64) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.WindowLimit = limit
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgUsersErrorWindow)
		return
	}

	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgUsersWindowSet, *user.Username, limit))
}

func (x *TelegramHandler) UsersCommandStack(log *tracing.Logger, msg *tgbotapi.Message, username string, enabled bool) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsStackAllowed = platform.BoolPtr(enabled)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgUsersErrorStack)
		return
	}

	status := "теперь доступен"
	if !enabled {
		status = "теперь недоступен"
	}
	x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgUsersStackSet, *user.Username, status))
}

func (x *TelegramHandler) retrieveUserByName(log *tracing.Logger, msg *tgbotapi.Message, username string) *entities.User {
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		x.diplomat.Reply(log, msg, texting.MsgUserNotSpecified)
		return nil
	}

	user, err := x.users.GetUserByName(log, username)
	if err != nil {
		x.diplomat.Reply(log, msg, fmt.Sprintf(texting.MsgUserNotFound, username))
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

// =========================  /donations command handlers  =========================

func (x *TelegramHandler) DonationsCommandAdd(log *tracing.Logger, msg *tgbotapi.Message, username string, sum float64) {
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgUserNotSpecified))
		return
	}

	user, err := x.users.GetUserByName(log, username)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(fmt.Sprintf(texting.MsgUserNotFound, username)))
		return
	}

	_, err = x.donations.CreateDonation(log, user, sum)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgDonationsErrorAdd))
		return
	}

	x.diplomat.Reply(log, msg, texting.XiifyManual(fmt.Sprintf(texting.MsgDonationsAdded, *user.Username, texting.DecimalifyFloat(sum))))
}

func (x *TelegramHandler) DonationsCommandList(log *tracing.Logger, msg *tgbotapi.Message) {
	donations, err := x.donations.GetDonationsWithUsers(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgDonationsErrorList))
		return
	}

	if len(donations) == 0 {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgDonationsNoDonations))
		return
	}

	var builder strings.Builder
	builder.WriteString(texting.MsgDonationsListHeader)

	for _, donation := range donations {
		if donation.Sum.IsZero() {
			continue
		}

		username := "Мертвая душа"
		if donation.UserEntity.Username != nil {
			username = "@" + *donation.UserEntity.Username
		}

		builder.WriteString(fmt.Sprintf(
			texting.MsgDonationsListItem,
			username,
			texting.Decimalify(donation.Sum),
			donation.CreatedAt.Format("02.01.2006"),
		))
	}

	x.diplomat.Reply(log, msg, builder.String())
}

// =========================  /stats command handlers  =========================

func (x *TelegramHandler) StatsCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	totalQuestions, err := x.messages.GetTotalUserQuestionsCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	chatQuestions, err := x.messages.GetUserQuestionsInChatCount(log, msg.Chat.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	userQuestions, err := x.messages.GetUserPersonalQuestionsCount(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	userChatQuestions, err := x.messages.GetUserPersonalQuestionsInChatCount(log, user, msg.Chat.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	totalUsers, err := x.users.GetTotalUsersCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	totalChats, err := x.messages.GetUniqueChatCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	totalCost, err := x.usage.GetTotalCost(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	totalCostLastMonth, err := x.usage.GetTotalCostLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	avgDailyCost, err := x.usage.GetAverageDailyCost(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	totalTokens, err := x.usage.GetTotalTokens(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	totalTokensLastMonth, err := x.usage.GetTotalTokensLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	userCost, err := x.usage.GetUserCost(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	userCostLastMonth, err := x.usage.GetUserCostLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	userAvgDailyCost, err := x.usage.GetUserAverageDailyCost(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	userTokens, err := x.usage.GetUserTokens(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	userTokensLastMonth, err := x.usage.GetUserTokensLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgStatsError))
		return
	}

	response := texting.MsgStatsTitle +
		fmt.Sprintf(texting.MsgStatsGeneral, 
			texting.Numberify(totalQuestions), 
			texting.Numberify(chatQuestions),
			texting.CurrencifyDecimal(totalCost),
			texting.CurrencifyDecimal(totalCostLastMonth),
			texting.CurrencifyDecimal(avgDailyCost),
			texting.Numberify(totalTokens),
			texting.Numberify(totalTokensLastMonth)) +
		fmt.Sprintf(texting.MsgStatsPersonal, 
			texting.Numberify(userQuestions), 
			texting.Numberify(userChatQuestions),
			texting.CurrencifyDecimal(userCost),
			texting.CurrencifyDecimal(userCostLastMonth),
			texting.CurrencifyDecimal(userAvgDailyCost),
			texting.Numberify(userTokens),
			texting.Numberify(userTokensLastMonth)) +
		fmt.Sprintf(texting.MsgStatsUsers, 
			texting.Numberify(totalUsers), 
			texting.Numberify(totalChats))

	x.diplomat.Reply(log, msg, texting.XiifyManual(response))
}

// =========================  /context command handlers  =========================

func (x *TelegramHandler) ContextCommandRefresh(log *tracing.Logger, msg *tgbotapi.Message, chatID int64) {
	err := x.messages.MarkChatMessagesAsRemoved(log, chatID, time.Now())
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextErrorRefresh))
		return
	}

	x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextRefreshed))
}

func (x *TelegramHandler) ContextCommandDisable(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	user.IsStackEnabled = platform.BoolPtr(false)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextErrorDisable))
		return
	}

	x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextDisabled))
}

func (x *TelegramHandler) ContextCommandEnable(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	user.IsStackEnabled = platform.BoolPtr(true)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextErrorEnable))
		return
	}

	x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextEnabled))
}

// =========================  /wtf command handlers  =========================

// =========================  /pinned command handlers  =========================

func (x *TelegramHandler) PinnedCommandAdd(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, message string) {
	if len(message) > 1024 {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedTooLong))
		return
	}

	donations, err := x.donations.GetDonationsByUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedErrorAdd))
		return
	}

	err = x.pins.CheckPinLimit(log, user, msg.Chat.ID, donations)
	if err != nil {
		if errors.Is(err, repository.ErrPinLimitExceeded) {
			if len(donations) > 0 {
				x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedFinalLimitReached))
			} else {
				x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedLimitReached))
			}
			return
		}

		if errors.Is(err, repository.ErrPinLimitExceededChat) {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedChatLimitReached))
			return
		}

		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedErrorAdd))
		return
	}

	_, err = x.pins.CreatePin(log, user, msg.Chat.ID, message)
	if err != nil {
		if errors.Is(err, repository.ErrPinTooLong) {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedTooLong))
			return
		}

		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedErrorAdd))
		return
	}

	x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedAdded))
}

func (x *TelegramHandler) PinnedCommandRemove(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, message string) {
	_, err := x.pins.GetPinByUserChatAndMessage(log, user, msg.Chat.ID, message)
	if err != nil {
		if errors.Is(err, repository.ErrPinNotFound) {
			x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedNotFound))
			return
		}

		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedErrorRemove))
		return
	}

	err = x.pins.DeletePin(log, user, msg.Chat.ID, message)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedErrorRemove))
		return
	}

	x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedRemoved))
}

func (x *TelegramHandler) PinnedCommandList(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	pins, err := x.pins.GetPinsByChat(log, msg.Chat.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedErrorList))
		return
	}

	if len(pins) == 0 {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgPinnedListEmpty))
		return
	}

	response := texting.MsgPinnedListHeader
	for i, pin := range pins {
		authorName := "Мертвая душа"
		if pin.UserEntity.Fullname != nil && *pin.UserEntity.Fullname != "" {
			authorName = *pin.UserEntity.Fullname
		}
		username := ""
		if pin.UserEntity.Username != nil && *pin.UserEntity.Username != "" {
			username = " (@" + *pin.UserEntity.Username + ")"
		}
		response += fmt.Sprintf("%d. %s%s: %s\n", i+1, authorName, username, pin.Message)
	}

	donations, _ := x.donations.GetDonationsByUser(log, user)
	userPinsCount, _ := x.pins.CountPinsByUserInChat(log, user, msg.Chat.ID)

	limit := x.pins.GetPinLimit(log, user, donations)
	response += fmt.Sprintf("\n" + texting.MsgPinnedListFooter, userPinsCount, limit)

	x.diplomat.Reply(log, msg, response)
}