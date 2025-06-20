package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
	"ximanager/sources/artificial"
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

	response, err := x.orchestrator.Orchestrate(log, msg, req)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgErrorResponse)
		return
	}

	x.diplomat.Reply(log, msg, texting.Xiify(response))
}

func (x *TelegramHandler) XiCommandPhoto(log *tracing.Logger, msg *tgbotapi.Message) {
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

	oai := x.balancer.GetNeuroProviderByName("openai").(*artificial.OpenAIClient)
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	response, err := oai.ResponseImage(ctx, log, iurl, req, msg.From.FirstName+" "+msg.From.LastName)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.MsgErrorResponse)
		return
	}

	x.diplomat.Reply(log, msg, texting.Xiify(response))
}

func (x *TelegramHandler) XiCommandAudio(log *tracing.Logger, msg *tgbotapi.Message, replyMsg *tgbotapi.Message) {
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
	
	oai := x.balancer.GetNeuroProviderByName("openai").(*artificial.OpenAIClient)
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 2*time.Minute)
	defer cancel()

	transcriptedText, err := oai.ResponseAudio(ctx, log, tempFile, "")
	if err != nil {
		log.E("Error transcribing audio", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, texting.MsgAudioError)
		return
	}

	if userPrompt != "" {
		response, err := oai.ResponseLightweight(ctx, log, transcriptedText, userPrompt, msg.From.FirstName+" "+msg.From.LastName)
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

func (x *TelegramHandler) ModeCommandAdd(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, chatID int64, modeType string, modeName string, prompt string) {
	mode, err := x.modes.AddModeForChat(log, chatID, modeType, modeName, prompt, msg.From.ID)
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
	
	mode.IsEnabled = false
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
	
	mode.IsEnabled = true
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

func (x *TelegramHandler) ModeCommandEdit(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string, prompt string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	mode.Prompt = prompt
	_, err := x.modes.UpdateMode(log, mode)
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

	user.IsActive = false
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

	user.IsActive = true
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

	user.IsStackAllowed = enabled
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

	x.diplomat.Reply(log, msg, texting.XiifyManual(fmt.Sprintf(texting.MsgDonationsAdded, *user.Username, sum)))
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

	response := texting.MsgStatsTitle +
		fmt.Sprintf(texting.MsgStatsGeneral, totalQuestions, chatQuestions) +
		fmt.Sprintf(texting.MsgStatsPersonal, userQuestions, userChatQuestions) +
		fmt.Sprintf(texting.MsgStatsUsers, totalUsers, totalChats)

	x.diplomat.Reply(log, msg, texting.XiifyManual(response))
}

// =========================  /context command handlers  =========================

func (x *TelegramHandler) ContextCommandRefresh(log *tracing.Logger, msg *tgbotapi.Message, chatID int64) {
	err := x.messages.MarkChatMessagesAsRemoved(log, chatID, time.Now())
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextError))
		return
	}

	x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgContextRefreshed))
}

// =========================  /wtf command handlers  =========================

func (x *TelegramHandler) WtfCommand(log *tracing.Logger, msg *tgbotapi.Message) {
	wtfDaysBack := platform.GetAsInt("WTF_DAYS_BACK", 2)
	wtfMessageLimit := platform.GetAsInt("WTF_MESSAGE_LIMIT", 80)

	fromTime := time.Now().AddDate(0, 0, -wtfDaysBack)

	messages, err := x.messages.GetRecentUserQuestions(log, msg.Chat.ID, fromTime, wtfMessageLimit)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgWtfError))
		return
	}

	if len(messages) == 0 {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgWtfNoQuestions))
		return
	}

	var questionsText strings.Builder
	for i, message := range messages {
		questionsText.WriteString(fmt.Sprintf("%d. %s\n", i+1, string(message.MessageText)))
	}

	oai := x.balancer.GetNeuroProviderByName("openai").(*artificial.OpenAIClient)
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 1*time.Minute)
	defer cancel()

	response, err := oai.ResponseLightweight(ctx, log, questionsText.String(), texting.MsgWtfPrompt, msg.From.FirstName+" "+msg.From.LastName)
	if err != nil {
		x.diplomat.Reply(log, msg, texting.XiifyManual(texting.MsgWtfError))
		return
	}

	finalResponse := texting.MsgWtfTitle + response
	x.diplomat.Reply(log, msg, texting.XiifyManual(finalResponse))
}
