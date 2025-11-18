package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting/format"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// =========================  /xi command handlers  =========================

func (x *TelegramHandler) XiCommandText(log *tracing.Logger, msg *tgbotapi.Message) {
	req := x.GetRequestText(msg)
	if req == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgHelpText")))
		return
	}

	x.diplomat.SendTyping(log, msg.Chat.ID)

  persona := msg.From.FirstName+" "+msg.From.LastName + " (@" + msg.From.UserName + ")"
	response, err := x.dialer.Dial(log, msg, req, persona, true)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgErrorResponse"))
		return
	}

	if strings.TrimSpace(response) == "" {
		log.W("Empty response from AI orchestrator", "response", response)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgErrorResponse"))
		return
	}

	x.diplomat.Reply(log, msg, x.personality.Xiify(msg, response))
}

func (x *TelegramHandler) XiCommandPhoto(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	x.diplomat.SendTyping(log, msg.Chat.ID)

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
	response, err := x.vision.Visionify(log, iurl, user, msg.Chat.ID, req, persona)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgErrorResponse"))
		return
	}

	x.diplomat.Reply(log, msg, x.personality.Xiify(msg, response))
}

func (x *TelegramHandler) XiCommandAudio(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, replyMsg *tgbotapi.Message) {
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
	transcriptedText, err := x.whisper.Whisperify(log, tempFile, user)
	if err != nil {
		log.E("Error transcribing audio", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgAudioError"))
		return
	}

	if userPrompt != "" {
    persona := msg.From.FirstName+" "+msg.From.LastName + " (@" + msg.From.UserName + ")"
		response, err := x.dialer.Dial(log, msg, transcriptedText, persona, false)
		if err != nil {
			log.E("Error processing with lightweight model", tracing.InnerError, err)
			x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgAudioError"))
			return
		}
		x.diplomat.Reply(log, msg, x.personality.XiifyAudio(msg, response))
	} else {
		x.diplomat.Reply(log, msg, x.personality.XiifyAudio(msg, transcriptedText))
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
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorSwitching"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgModeChanged", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	}))
}

func (x *TelegramHandler) ModeCommandAdd(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, chatID int64, modeType string, modeName string, configJSON string) {
	var config repository.ModeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeErrorConfigParse")))
		return
	}

	if strings.TrimSpace(config.Prompt) == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeErrorConfigPrompt")))
		return
	}

	mode, err := x.modes.AddModeWithConfig(log, chatID, modeType, modeName, &config, msg.From.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorAdd"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgModeAdded", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	}))
}

func (x *TelegramHandler) ModeCommandList(log *tracing.Logger, msg *tgbotapi.Message, chatID int64) {
	modes, err := x.modes.GetModesByChat(log, chatID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorGettingList"))
		return
	}

	if len(modes) == 0 {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeNoModesAvailable"))
		return
	}

	currentMode, err := x.modes.GetModeByChat(log, chatID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorGettingList"))
		return
	}

	message := x.localization.LocalizeBy(msg, "MsgModeListHeader")
	for _, mode := range modes {
		if mode.ID == currentMode.ID {
			message += x.localization.LocalizeByTd(msg, "MsgModeListItemCurrent", map[string]interface{}{
				"Name": mode.Name,
				"Type": mode.Type,
			})
		} else {
			message += x.localization.LocalizeByTd(msg, "MsgModeListItem", map[string]interface{}{
				"Name": mode.Name,
				"Type": mode.Type,
			})
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
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorDisable"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgModeDisabled", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	}))
}

func (x *TelegramHandler) ModeCommandEnable(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	mode.IsEnabled = platform.BoolPtr(true)
	_, err := x.modes.UpdateMode(log, mode)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorEnable"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgModeEnabled", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	}))
}

func (x *TelegramHandler) ModeCommandDelete(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	err := x.modes.DeleteMode(log, mode)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorDelete"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgModeDeleted", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	}))
}

func (x *TelegramHandler) ModeCommandEdit(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string, configJSON string) {
	mode := x.retrieveModeByChat(log, msg, chatID, modeType)
	if mode == nil {
		return
	}

	var config repository.ModeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeErrorConfigParse")))
		return
	}

	if strings.TrimSpace(config.Prompt) == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgModeErrorConfigPrompt")))
		return
	}

	err := x.modes.UpdateModeConfig(log, mode.ID, &config)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgModeErrorEdit"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgModeEdited", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	}))
}

func (x *TelegramHandler) retrieveModeByChat(log *tracing.Logger, msg *tgbotapi.Message, chatID int64, modeType string) *entities.Mode {
	mode, err := x.modes.GetModeByChat(log, chatID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgModeNotFoundGeneral", map[string]interface{}{
			"Mode": modeType,
		}))
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
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersErrorEdit"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgUsersEdited", map[string]interface{}{
		"Username": *user.Username,
		"Rights":   strings.Join(rights, ", "),
	}))
}

func (x *TelegramHandler) UsersCommandDisable(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsActive = platform.BoolPtr(false)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersErrorDisable"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgUsersDisabled", map[string]interface{}{
		"Username": *user.Username,
	}))
}

func (x *TelegramHandler) UsersCommandEnable(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsActive = platform.BoolPtr(true)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersErrorEnable"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgUsersEnabled", map[string]interface{}{
		"Username": *user.Username,
	}))
}

func (x *TelegramHandler) UsersCommandWindow(log *tracing.Logger, msg *tgbotapi.Message, username string, limit int64) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.WindowLimit = limit
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersErrorWindow"))
		return
	}

	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgUsersWindowSet", map[string]interface{}{
		"Username": *user.Username,
		"Window":   limit,
	}))
}

func (x *TelegramHandler) UsersCommandStack(log *tracing.Logger, msg *tgbotapi.Message, username string, enabled bool) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	user.IsStackAllowed = platform.BoolPtr(enabled)
	_, err := x.users.UpdateUser(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgUsersErrorStack"))
		return
	}

	status := "теперь доступен"
	if !enabled {
		status = "теперь недоступен"
	}
	x.diplomat.Reply(log, msg, x.localization.LocalizeByTd(msg, "MsgUsersStackSet", map[string]interface{}{
		"Username": *user.Username,
		"Status":   status,
	}))
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

// =========================  /donations command handlers  =========================

func (x *TelegramHandler) DonationsCommandAdd(log *tracing.Logger, msg *tgbotapi.Message, username string, sum float64) {
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgUserNotSpecified")))
		return
	}

	user, err := x.users.GetUserByName(log, username)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeByTd(msg, "MsgUserNotFound", map[string]interface{}{
			"Username": username,
		})))
		return
	}

	_, err = x.donations.CreateDonation(log, user, sum)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgDonationsErrorAdd")))
		return
	}

	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeByTd(msg, "MsgDonationsAdded", map[string]interface{}{
		"Username": *user.Username,
		"Amount":   format.DecimalifyFloat(sum),
	})))
}

func (x *TelegramHandler) DonationsCommandList(log *tracing.Logger, msg *tgbotapi.Message) {
	donations, err := x.donations.GetDonationsWithUsers(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgDonationsErrorList")))
		return
	}

	if len(donations) == 0 {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgDonationsNoDonations")))
		return
	}

	userDonations := make(map[uuid.UUID]decimal.Decimal)
	usersMap := make(map[uuid.UUID]entities.User)

	for _, donation := range donations {
		if donation.Sum.IsZero() {
			continue
		}
		userDonations[donation.User] = userDonations[donation.User].Add(donation.Sum)
		if _, ok := usersMap[donation.User]; !ok {
			usersMap[donation.User] = donation.UserEntity
		}
	}

	type userDonation struct {
		User  entities.User
		Total decimal.Decimal
	}

	var sortedDonations []userDonation
	for userID, total := range userDonations {
		sortedDonations = append(sortedDonations, userDonation{
			User:  usersMap[userID],
			Total: total,
		})
	}

	sort.Slice(sortedDonations, func(i, j int) bool {
		return sortedDonations[i].Total.GreaterThan(sortedDonations[j].Total)
	})

	var builder strings.Builder
	builder.WriteString(x.localization.LocalizeBy(msg, "MsgDonationsListHeader"))

	if len(sortedDonations) > 0 {
		builder.WriteString(x.localization.LocalizeBy(msg, "MsgDonationsListTopHeader"))
	}

	for i, ud := range sortedDonations {
		username := "Мертвая душа"
		if ud.User.Username != nil {
			username = "@" + *ud.User.Username
		}

		if i < 3 {
			var messageID string
			switch i {
			case 0:
				messageID = "MsgDonationsListTop1Item"
			case 1:
				messageID = "MsgDonationsListTop2Item"
			case 2:
				messageID = "MsgDonationsListTop3Item"
			}
			builder.WriteString(x.localization.LocalizeByTd(msg, messageID, map[string]interface{}{
				"Username": username,
				"Amount":   format.Decimalify(ud.Total),
			}))
		} else {
			if i == 3 {
				builder.WriteString(x.localization.LocalizeBy(msg, "MsgDonationsListOthersHeader"))
			}
			builder.WriteString(x.localization.LocalizeByTd(msg, "MsgDonationsListItem", map[string]interface{}{
				"Username": username,
				"Amount":   format.Decimalify(ud.Total),
			}))
		}
	}

	x.diplomat.Reply(log, msg, builder.String())
}

// =========================  /stats command handlers  =========================

func (x *TelegramHandler) StatsCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	totalQuestions, err := x.messages.GetTotalUserQuestionsCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	chatQuestions, err := x.messages.GetUserQuestionsInChatCount(log, msg.Chat.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	userQuestions, err := x.messages.GetUserPersonalQuestionsCount(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	userChatQuestions, err := x.messages.GetUserPersonalQuestionsInChatCount(log, user, msg.Chat.ID)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	totalUsers, err := x.users.GetTotalUsersCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	totalChats, err := x.messages.GetUniqueChatCount(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	totalCost, err := x.usage.GetTotalCost(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	totalCostLastMonth, err := x.usage.GetTotalCostLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	avgDailyCost, err := x.usage.GetAverageDailyCost(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	totalTokens, err := x.usage.GetTotalTokens(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	totalTokensLastMonth, err := x.usage.GetTotalTokensLastMonth(log)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	userCost, err := x.usage.GetUserCost(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	userCostLastMonth, err := x.usage.GetUserCostLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	userAvgDailyCost, err := x.usage.GetUserAverageDailyCost(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	userTokens, err := x.usage.GetUserTokens(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	userTokensLastMonth, err := x.usage.GetUserTokensLastMonth(log, user)
	if err != nil {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgStatsError")))
		return
	}

	response := x.localization.LocalizeBy(msg, "MsgStatsTitle") +
		x.localization.LocalizeByTd(msg, "MsgStatsGeneral", map[string]interface{}{
			"TotalQuestions": format.Numberify(totalQuestions),
			"ChatQuestions":  format.Numberify(chatQuestions),
			"TotalCost":       format.CurrencifyDecimal(totalCost),
			"MonthlyCost":    format.CurrencifyDecimal(totalCostLastMonth),
			"DailyCost":      format.CurrencifyDecimal(avgDailyCost),
			"TotalTokens":    format.Numberify(totalTokens),
			"MonthlyTokens":  format.Numberify(totalTokensLastMonth),
		}) +
		x.localization.LocalizeByTd(msg, "MsgStatsPersonal", map[string]interface{}{
			"TotalQuestions": format.Numberify(userQuestions),
			"ChatQuestions":  format.Numberify(userChatQuestions),
			"TotalCost":       format.CurrencifyDecimal(userCost),
			"MonthlyCost":    format.CurrencifyDecimal(userCostLastMonth),
			"DailyCost":      format.CurrencifyDecimal(userAvgDailyCost),
			"TotalTokens":    format.Numberify(userTokens),
			"MonthlyTokens":  format.Numberify(userTokensLastMonth),
		}) +
		x.localization.LocalizeByTd(msg, "MsgStatsUsers", map[string]interface{}{
			"Users": format.Numberify(totalUsers),
			"Chats": format.Numberify(totalChats),
		})

	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, response))
}

func (x *TelegramHandler) PersonalizationCommandSet(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, prompt string) {
	promptRunes := []rune(prompt)
	if len(promptRunes) < 12 {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationTooShort")))
		return
	}

	if len(promptRunes) > 1024 {
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationTooLong")))
		return
	}

	validation, err := x.agents.ValidatePersonalization(log, prompt)
	if err != nil {
		log.E("Failed to validate personalization with agent", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationValidationError")))
		return
	}

	if validation.Confidence <= 0.68 {
		log.I("Personalization validation failed", "confidence", validation.Confidence)
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationValidationFailed")))
		return
	}

	_, err = x.personalizations.CreateOrUpdatePersonalization(log, user, prompt)
	if err != nil {
		if errors.Is(err, repository.ErrPersonalizationTooShort) {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationTooShort")))
			return
		}

		if errors.Is(err, repository.ErrPersonalizationTooLong) {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationTooLong")))
			return
		}

		log.E("Failed to create personalization", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationErrorAdd")))
		return
	}

	log.I("Personalization set successfully", "confidence", validation.Confidence)
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationAdded")))
}

func (x *TelegramHandler) PersonalizationCommandRemove(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	err := x.personalizations.DeletePersonalization(log, user)
	if err != nil {
		if errors.Is(err, repository.ErrPersonalizationNotFound) {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationNotFound")))
			return
		}

		log.E("Failed to delete personalization", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationErrorRemove")))
		return
	}

	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationRemoved")))
}

func (x *TelegramHandler) PersonalizationCommandPrint(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	personalization, err := x.personalizations.GetPersonalizationByUser(log, user)
	if err != nil {
		if errors.Is(err, repository.ErrPersonalizationNotFound) {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationNotFound")))
			return
		}

		log.E("Failed to get personalization", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgPersonalizationErrorPrint")))
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
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgContextRefreshError")))
		return
	}

	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgContextRefreshed")))
}

// =========================  /ban and /pardon command handlers  =========================

func (x *TelegramHandler) BanCommandApply(log *tracing.Logger, msg *tgbotapi.Message, username string, reason string, duration string) {
	targetUser := x.retrieveUserByName(log, msg, username)
	if targetUser == nil {
		return
	}

	_, err := x.bans.ParseDuration(duration)
	if err != nil {
		if errors.Is(err, repository.ErrDurationTooLong) {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgBanErrorTooLong")))
		} else {
			x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgBanErrorInvalid")))
		}
		return
	}

	_, err = x.bans.CreateBan(log, targetUser.ID, msg.Chat.ID, reason, duration)
	if err != nil {
		log.E("Failed to create ban", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgBanErrorCreate")))
		return
	}

	displayName := *targetUser.Username
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeByTd(msg, "MsgBanApplied", map[string]interface{}{
		"Username": displayName,
		"Duration": duration,
		"Reason":   reason,
	})))
}

func (x *TelegramHandler) PardonCommandApply(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	targetUser := x.retrieveUserByName(log, msg, username)
	if targetUser == nil {
		return
	}

	_, err := x.bans.GetActiveBan(log, targetUser.ID)
	if err != nil {
		displayName := *targetUser.Username
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeByTd(msg, "MsgBanErrorNotFound", map[string]interface{}{
			"Username": displayName,
		})))
		return
	}

	err = x.bans.DeleteBansByUser(log, targetUser.ID)
	if err != nil {
		log.E("Failed to pardon user", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeBy(msg, "MsgBanErrorRemove")))
		return
	}

	displayName := *targetUser.Username
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, x.localization.LocalizeByTd(msg, "MsgBanPardon", map[string]interface{}{
		"Username": displayName,
	})))
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

	response := x.localization.LocalizeBy(msg, "MsgHealthTitle") +
		x.localization.LocalizeByTd(msg, "MsgHealthDatabase", map[string]interface{}{
			"Status": dbStatus,
		}) +
		x.localization.LocalizeByTd(msg, "MsgHealthRedis", map[string]interface{}{
			"Status": redisStatus,
		}) +
		x.localization.LocalizeByTd(msg, "MsgHealthSystem", map[string]interface{}{
			"Status": systemStatus,
		}) +
		x.localization.LocalizeByTd(msg, "MsgHealthUptime", map[string]interface{}{
			"Uptime": uptime,
		}) +
		x.localization.LocalizeByTd(msg, "MsgHealthVersion", map[string]interface{}{
			"Version": version,
		}) +
		x.localization.LocalizeByTd(msg, "MsgHealthBuildTime", map[string]interface{}{
			"BuildTime": buildTime,
		})

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
	accountAge := format.Ageify(user.CreatedAt)

	response := x.localization.LocalizeByTd(msg, "MsgThisInfo", map[string]interface{}{
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
	})

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