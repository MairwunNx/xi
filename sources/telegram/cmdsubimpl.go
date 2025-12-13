package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting/format"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
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

	response, err := x.dialer.Dial(log, msg, req, "", persona, true)
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

	iurl := fmt.Sprintf(GetFileAPIEndpoint(x.diplomat.config), x.diplomat.bot.Token, file.FilePath)

	req := ""

	if msg.IsCommand() {
		req = msg.CommandArguments()
	}

	if req == "" {
		req = msg.Caption
	}

	persona := msg.From.FirstName + " " + msg.From.LastName + " (@" + msg.From.UserName + ")"

	response, err := x.dialer.Dial(log, msg, req, iurl, persona, true)
	if err != nil {
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgErrorResponse"))
		return
	}

	x.diplomat.Reply(log, msg, x.personality.Xiify(msg, response))
}

func (x *TelegramHandler) XiCommandPhotoFromReply(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, replyMsg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Xi command photo from reply completed", "telegram.command.xi.photo_reply", "chat_id", msg.Chat.ID)()
	x.diplomat.StartTyping(msg.Chat.ID)
	defer x.diplomat.StopTyping(msg.Chat.ID)

	photo := replyMsg.Photo[len(replyMsg.Photo)-1]

	fileConfig := tgbotapi.FileConfig{FileID: photo.FileID}
	file, err := x.diplomat.bot.GetFile(fileConfig)
	if err != nil {
		log.E("Error getting file", tracing.InnerError, err)
		x.diplomat.Reply(log, msg, x.localization.LocalizeBy(msg, "MsgErrorResponse"))
		return
	}

	iurl := fmt.Sprintf(GetFileAPIEndpoint(x.diplomat.config), x.diplomat.bot.Token, file.FilePath)

	req := strings.TrimSpace(msg.CommandArguments())

	persona := msg.From.FirstName + " " + msg.From.LastName + " (@" + msg.From.UserName + ")"

	response, err := x.dialer.Dial(log, msg, req, iurl, persona, true)
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

	fileURL := fmt.Sprintf(GetFileAPIEndpoint(x.diplomat.config), x.diplomat.bot.Token, file.FilePath)

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

// =========================  /mode command handlers  =========================

// ModeCommandShowList shows the list of modes with selection buttons
func (x *TelegramHandler) ModeCommandShowList(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Mode command show list completed", "telegram.command.mode.show.list", "chat_id", msg.Chat.ID)()

	available, unavailable, _, err := x.modes.GetAllModesWithAvailability(log, user)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorGettingList")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	if len(available) == 0 && len(unavailable) == 0 {
		noModesMsg := x.localization.LocalizeBy(msg, "MsgModeNoModesAvailable")
		x.diplomat.Reply(log, msg, noModesMsg)
		return
	}

	currentMode, _ := x.modes.GetCurrentModeForChat(log, msg.Chat.ID)

	// Build message
	message := x.localization.LocalizeBy(msg, "MsgModeListTitle")

	if len(available) > 0 {
		message += x.localization.LocalizeBy(msg, "MsgModeListAvailable")
		for _, mode := range available {
			modeData := map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
			}
			if currentMode != nil && mode.ID == currentMode.ID {
				message += x.localization.LocalizeByTd(msg, "MsgModeListItemCurrent", modeData)
			} else {
				message += x.localization.LocalizeByTd(msg, "MsgModeListItemAvailable", modeData)
			}
		}
	}

	if len(unavailable) > 0 {
		message += x.localization.LocalizeBy(msg, "MsgModeListUnavailable")
		for _, mode := range unavailable {
			gradeName := x.getGradeDisplayName(msg, mode.Grade)
			modeData := map[string]interface{}{
				"Name":  mode.Name,
				"Grade": gradeName,
			}
			message += x.localization.LocalizeByTd(msg, "MsgModeListItemUnavailable", modeData)
		}
	}

	message += x.localization.LocalizeBy(msg, "MsgModeListFooter")

	// Build inline keyboard with mode buttons (only for available modes)
	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, mode := range available {
		btn := tgbotapi.NewInlineKeyboardButtonData(mode.Name, "mode_select_"+mode.Type)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(btn))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	x.diplomat.ReplyWithKeyboard(log, msg, message, keyboard)
}

func (x *TelegramHandler) ModeCommandCreateStart(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Mode command create start completed", "telegram.command.mode.create.start", "chat_id", msg.Chat.ID)()

	err := x.chatState.InitModeCreation(log, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.E("Failed to init mode creation state", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorAdd")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	startMsg := x.localization.LocalizeBy(msg, "MsgModeCreateStart")
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, startMsg))
}

func (x *TelegramHandler) ModeCommandEditStart(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, modeType string) {
	defer tracing.ProfilePoint(log, "Mode command edit start completed", "telegram.command.mode.edit.start", "chat_id", msg.Chat.ID, "mode_type", modeType)()

	mode, err := x.modes.GetModeByTypeIncludingDisabled(log, modeType)
	if err != nil {
		errorMsg := x.localization.LocalizeByTd(msg, "MsgModeNotFound", map[string]interface{}{
			"Type": modeType,
		})
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	// Build edit menu
	editTitle := x.localization.LocalizeByTd(msg, "MsgModeEditTitle", map[string]interface{}{
		"Name": mode.Name,
	})

	// Build inline keyboard
	nameBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgModeEditNameBtn"),
		"mode_edit_name_"+mode.Type,
	)
	promptBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgModeEditPromptBtn"),
		"mode_edit_prompt_"+mode.Type,
	)
	configBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgModeEditConfigBtn"),
		"mode_edit_config_"+mode.Type,
	)

	var toggleBtn tgbotapi.InlineKeyboardButton
	if platform.BoolValue(mode.IsEnabled, true) {
		toggleBtn = tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgModeEditDisableBtn"),
			"mode_edit_disable_"+mode.Type,
		)
	} else {
		toggleBtn = tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgModeEditEnableBtn"),
			"mode_edit_enable_"+mode.Type,
		)
	}

	deleteBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgModeEditDeleteBtn"),
		"mode_edit_delete_"+mode.Type,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(nameBtn, promptBtn),
		tgbotapi.NewInlineKeyboardRow(configBtn, toggleBtn),
		tgbotapi.NewInlineKeyboardRow(deleteBtn),
	)

	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManual(msg, editTitle), keyboard)
}

func (x *TelegramHandler) ModeCommandInfoList(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Mode command info list completed", "telegram.command.mode.info.list", "chat_id", msg.Chat.ID)()

	modes, err := x.modes.GetAllModesIncludingDisabled(log)
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

	infoTitle := x.localization.LocalizeBy(msg, "MsgModeInfoTitle")

	// Build inline keyboard with mode buttons
	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, mode := range modes {
		btn := tgbotapi.NewInlineKeyboardButtonData(mode.Name, "mode_info_"+mode.Type)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(btn))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManual(msg, infoTitle), keyboard)
}

func (x *TelegramHandler) handleChatStateMessage(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) bool {
	state, err := x.chatState.GetState(log, msg.Chat.ID, msg.From.ID)
	if err != nil || state == nil || state.Status == repository.ChatStateNone {
		return false
	}

	// Only process messages from the user who initiated the wizard
	if state.UserID != msg.From.ID {
		return false
	}

	log.I("Processing chat state message", "status", repository.GetStatusName(state.Status))

	switch state.Status {
	case repository.ChatStateAwaitingModeType:
		x.handleModeTypeInput(log, user, msg, state)
		return true
	case repository.ChatStateAwaitingModeName:
		x.handleModeNameInput(log, user, msg, state)
		return true
	case repository.ChatStateAwaitingGrade:
		x.handleModeGradeInput(log, user, msg, state)
		return true
	case repository.ChatStateAwaitingPrompt:
		x.handlePromptInput(log, user, msg, state)
		return true
	case repository.ChatStateAwaitingConfig:
		x.handleConfigInput(log, user, msg, state)
		return true
	case repository.ChatStateAwaitingNewName:
		x.handleNewNameInput(log, user, msg, state)
		return true
	case repository.ChatStateAwaitingPersonalization:
		x.handlePersonalizationInput(log, user, msg)
		return true
	case repository.ChatStateAwaitingBroadcast:
		x.handleBroadcastInput(log, user, msg)
		return true
	case repository.ChatStateAwaitingTariffKey:
		x.handleTariffKeyInput(log, user, msg, state)
		return true
	case repository.ChatStateAwaitingTariffConfig:
		x.handleTariffConfigInput(log, user, msg, state)
		return true
	}

	return false
}

func (x *TelegramHandler) handleModeTypeInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	modeType := strings.TrimSpace(msg.Text)

	if !isValidModeType(modeType) {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeCreateInvalidType")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	existingMode, _ := x.modes.GetModeByType(log, modeType)
	if existingMode != nil {
		warnMsg := x.localization.LocalizeByTd(msg, "MsgModeCreateTypeExists", map[string]interface{}{
			"Type": modeType,
		})
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, warnMsg))
	}

	// Move to next step
	err := x.chatState.SetModeType(log, msg.Chat.ID, msg.From.ID, modeType)
	if err != nil {
		log.E("Failed to set mode type in state", tracing.InnerError, err)
		return
	}

	nextMsg := x.localization.LocalizeByTd(msg, "MsgModeCreateAwaitingName", map[string]interface{}{
		"Type": modeType,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, nextMsg))
}

func (x *TelegramHandler) handleModeNameInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	modeName := strings.TrimSpace(msg.Text)

	if len(modeName) < 2 || len(modeName) > 100 {
		return // Invalid name, just ignore
	}

	// Move to next step (grade selection)
	err := x.chatState.SetModeName(log, msg.Chat.ID, msg.From.ID, modeName)
	if err != nil {
		log.E("Failed to set mode name in state", tracing.InnerError, err)
		return
	}

	nextMsg := x.localization.LocalizeByTd(msg, "MsgModeCreateAwaitingGrade", map[string]interface{}{
		"Type": state.ModeType,
		"Name": modeName,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, nextMsg))
}

func (x *TelegramHandler) handleModeGradeInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	gradeInput := strings.TrimSpace(strings.ToLower(msg.Text))

	var grade string
	switch gradeInput {
	case "all", "все", "全部", "-":
		grade = ""
	case "bronze", "бронза", "бронзовый", "青铜":
		grade = platform.GradeBronze
	case "silver", "серебро", "серебряный", "白银":
		grade = platform.GradeSilver
	case "gold", "золото", "золотой", "黄金":
		grade = platform.GradeGold
	default:
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeCreateInvalidGrade")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	// Move to next step
	err := x.chatState.SetModeGrade(log, msg.Chat.ID, msg.From.ID, grade)
	if err != nil {
		log.E("Failed to set mode grade in state", tracing.InnerError, err)
		return
	}

	gradeName := x.getGradeDisplayName(msg, &grade)

	nextMsg := x.localization.LocalizeByTd(msg, "MsgModeCreateAwaitingPrompt", map[string]interface{}{
		"Type":  state.ModeType,
		"Name":  state.ModeName,
		"Grade": gradeName,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, nextMsg))
}

func (x *TelegramHandler) handlePromptInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	prompt := strings.TrimSpace(msg.Text)

	if len(prompt) < 10 {
		return // Too short, ignore
	}

	// If editing existing mode
	if state.ModeID != "" {
		modeID, err := uuid.Parse(state.ModeID)
		if err != nil {
			log.E("Failed to parse mode ID", tracing.InnerError, err)
			return
		}

		err = x.modes.UpdateModePrompt(log, modeID, prompt)
		if err != nil {
			errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorEdit")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

		mode, _ := x.modes.GetModeByID(log, modeID)
		modeName := state.ModeName
		if mode != nil {
			modeName = mode.Name
		}

		// Clear state
		x.chatState.ClearState(log, msg.Chat.ID, msg.From.ID)

		successMsg := x.localization.LocalizeByTd(msg, "MsgModePromptUpdated", map[string]interface{}{
			"Name": modeName,
		})
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
		return
	}

	// Creating new mode
	config := repository.DefaultModeConfig(prompt)

	newMode, err := x.modes.CreateMode(log, state.ModeType, state.ModeName, config, state.ModeGrade, msg.From.ID)
	if err != nil {
		log.E("Failed to create mode", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorAdd")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	// Clear state
	x.chatState.ClearState(log, msg.Chat.ID, msg.From.ID)

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeAdded", map[string]interface{}{
		"Name": newMode.Name,
		"Type": newMode.Type,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) handleConfigInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	configJSON := strings.TrimSpace(msg.Text)

	if state.ModeID == "" {
		return // No mode to edit
	}

	modeID, err := uuid.Parse(state.ModeID)
	if err != nil {
		log.E("Failed to parse mode ID", tracing.InnerError, err)
		return
	}

	// Parse config JSON
	var params repository.AIParams
	if err := json.Unmarshal([]byte(configJSON), &params); err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorConfigParse")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	// Get current mode config and update params
	mode, err := x.modes.GetModeByID(log, modeID)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorEdit")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	config := x.modes.ParseModeConfig(mode, log)
	config.Params = &params

	err = x.modes.UpdateModeConfig(log, modeID, config)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorEdit")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	// Clear state
	x.chatState.ClearState(log, msg.Chat.ID, msg.From.ID)

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeConfigUpdated", map[string]interface{}{
		"Name": mode.Name,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) handleNewNameInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	newName := strings.TrimSpace(msg.Text)

	if len(newName) < 2 || len(newName) > 100 {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorNameLength")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	if state.ModeID == "" {
		return
	}

	modeID, err := uuid.Parse(state.ModeID)
	if err != nil {
		log.E("Failed to parse mode ID", tracing.InnerError, err)
		return
	}

	mode, err := x.modes.GetModeByID(log, modeID)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorEdit")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	oldName := mode.Name
	err = x.modes.UpdateModeName(log, modeID, newName)
	if err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgModeErrorEdit")
		x.diplomat.Reply(log, msg, errorMsg)
		return
	}

	x.chatState.ClearState(log, msg.Chat.ID, msg.From.ID)

	successMsg := x.localization.LocalizeByTd(msg, "MsgModeNameUpdated", map[string]interface{}{
		"OldName": oldName,
		"NewName": newName,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func (x *TelegramHandler) handleModeSelectCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	// Extract mode type from callback data: mode_select_{modeType}
	modeType := strings.TrimPrefix(query.Data, "mode_select_")

	mode, err := x.modes.GetModeByType(log, modeType)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeNotFound", map[string]interface{}{
			"Type": modeType,
		}))
		x.diplomat.bot.Request(callback)
		return
	}

	// Check if user has access to this mode
	available, _, _, err := x.modes.GetAllModesWithAvailability(log, user)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeErrorSwitching"))
		x.diplomat.bot.Request(callback)
		return
	}

	isAvailable := false
	for _, m := range available {
		if m.Type == modeType {
			isAvailable = true
			break
		}
	}

	if !isAvailable {
		gradeName := x.getGradeDisplayName(query.Message, mode.Grade)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeNotAvailableForGrade", map[string]interface{}{
			"Name":          mode.Name,
			"RequiredGrade": gradeName,
		}))
		x.diplomat.bot.Request(callback)
		return
	}

	// Check if already selected
	currentMode, _ := x.modes.GetCurrentModeForChat(log, query.Message.Chat.ID)
	if currentMode != nil && currentMode.Type == modeType {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeAlreadySelected"))
		x.diplomat.bot.Request(callback)
		return
	}

	// Set the mode
	err = x.modes.SetModeForChat(log, query.Message.Chat.ID, mode.ID, user.ID)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeErrorSwitching"))
		x.diplomat.bot.Request(callback)
		return
	}

	// Send callback
	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeChangedCallback", map[string]interface{}{
		"Name": mode.Name,
	}))
	x.diplomat.bot.Request(callback)

	// Send message
	successMsg := x.localization.LocalizeByTd(query.Message, "MsgModeChanged", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
	x.diplomat.SendMessage(log, query.Message.Chat.ID, successMsg)
}

func (x *TelegramHandler) handleModeEditCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	// Check permissions
	if !x.rights.IsUserHasRight(log, user, "edit_mode") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeModifyNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	// Parse callback data: mode_edit_{action}_{modeType}
	parts := strings.SplitN(strings.TrimPrefix(query.Data, "mode_edit_"), "_", 2)
	if len(parts) != 2 {
		return
	}

	action := parts[0]
	modeType := parts[1]

	mode, err := x.modes.GetModeByTypeIncludingDisabled(log, modeType)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeNotFound", map[string]interface{}{
			"Type": modeType,
		}))
		x.diplomat.bot.Request(callback)
		return
	}

	switch action {
	case "name":
		// Start name edit wizard
		err = x.chatState.InitNameEdit(log, query.Message.Chat.ID, query.From.ID, mode.ID)
		if err != nil {
			log.E("Failed to init name edit state", tracing.InnerError, err)
			return
		}

		callback := tgbotapi.NewCallback(query.ID, "")
		x.diplomat.bot.Request(callback)

		nameMsg := x.localization.LocalizeByTd(query.Message, "MsgModeAwaitingName", map[string]interface{}{
		"Name": mode.Name,
		})
		x.diplomat.SendMessage(log, query.Message.Chat.ID, x.personality.XiifyManualPlain(nameMsg))

	case "prompt":
		// Start prompt edit wizard
		err = x.chatState.InitPromptEdit(log, query.Message.Chat.ID, query.From.ID, mode.ID)
		if err != nil {
			log.E("Failed to init prompt edit state", tracing.InnerError, err)
			return
		}

		callback := tgbotapi.NewCallback(query.ID, "")
		x.diplomat.bot.Request(callback)

		promptMsg := x.localization.LocalizeByTd(query.Message, "MsgModeAwaitingPrompt", map[string]interface{}{
			"Name": mode.Name,
		})
		x.diplomat.SendMessage(log, query.Message.Chat.ID, x.personality.XiifyManualPlain(promptMsg))

	case "config":
		// Start config edit wizard
		err = x.chatState.InitConfigEdit(log, query.Message.Chat.ID, query.From.ID, mode.ID)
		if err != nil {
			log.E("Failed to init config edit state", tracing.InnerError, err)
		return
	}

		callback := tgbotapi.NewCallback(query.ID, "")
		x.diplomat.bot.Request(callback)

		configMsg := x.localization.LocalizeByTd(query.Message, "MsgModeAwaitingConfig", map[string]interface{}{
			"Name": mode.Name,
		})
		x.diplomat.SendMessage(log, query.Message.Chat.ID, x.personality.XiifyManualPlain(configMsg))

	case "disable":
		mode.IsEnabled = platform.BoolPtr(false)
		_, err = x.modes.UpdateMode(log, mode)
	if err != nil {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeErrorDisable"))
			x.diplomat.bot.Request(callback)
		return
	}

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeDisabled", map[string]interface{}{
			"Name": mode.Name,
			"Type": mode.Type,
		}))
		x.diplomat.bot.Request(callback)

		successMsg := x.localization.LocalizeByTd(query.Message, "MsgModeDisabled", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
		enableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(query.Message, "MsgModeEditEnableBtn"),
			"mode_edit_enable_"+mode.Type,
		)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(enableBtn))
		x.diplomat.SendMessageWithKeyboard(log, query.Message.Chat.ID, successMsg, keyboard)

	case "enable":
		mode.IsEnabled = platform.BoolPtr(true)
		_, err = x.modes.UpdateMode(log, mode)
		if err != nil {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeErrorEnable"))
			x.diplomat.bot.Request(callback)
			return
		}

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeEnabled", map[string]interface{}{
			"Name": mode.Name,
			"Type": mode.Type,
		}))
		x.diplomat.bot.Request(callback)

		successMsg := x.localization.LocalizeByTd(query.Message, "MsgModeEnabled", map[string]interface{}{
			"Name": mode.Name,
			"Type": mode.Type,
		})
		disableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(query.Message, "MsgModeEditDisableBtn"),
			"mode_edit_disable_"+mode.Type,
		)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(disableBtn))
		x.diplomat.SendMessageWithKeyboard(log, query.Message.Chat.ID, successMsg, keyboard)

	case "delete":
		callback := tgbotapi.NewCallback(query.ID, "")
		x.diplomat.bot.Request(callback)

		confirmMsg := x.localization.LocalizeByTd(query.Message, "MsgModeDeleteConfirm", map[string]interface{}{
			"Name": mode.Name,
		})

		cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(query.Message, "MsgModeDeleteCancelBtn"),
			"mode_delete_cancel_"+mode.Type,
		)
		confirmBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(query.Message, "MsgModeDeleteConfirmBtn"),
			"mode_delete_confirm_"+mode.Type,
		)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(cancelBtn, confirmBtn))

		x.diplomat.SendMessageWithKeyboard(log, query.Message.Chat.ID, x.personality.XiifyManualPlain(confirmMsg), keyboard)
	}
}

func (x *TelegramHandler) handleModeDeleteCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	if !x.rights.IsUserHasRight(log, user, "edit_mode") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeModifyNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	// Parse callback data: mode_delete_{confirm|cancel}_{modeType}
	parts := strings.SplitN(strings.TrimPrefix(query.Data, "mode_delete_"), "_", 2)
	if len(parts) != 2 {
		return
	}

	action := parts[0]
	modeType := parts[1]

	mode, err := x.modes.GetModeByTypeIncludingDisabled(log, modeType)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeNotFound", map[string]interface{}{
			"Type": modeType,
		}))
		x.diplomat.bot.Request(callback)
		return
	}

	switch action {
	case "cancel":
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeDeleteCancelled", map[string]interface{}{
			"Name": mode.Name,
		}))
		x.diplomat.bot.Request(callback)

		cancelMsg := x.localization.LocalizeByTd(query.Message, "MsgModeDeleteCancelled", map[string]interface{}{
			"Name": mode.Name,
		})
		x.diplomat.SendMessage(log, query.Message.Chat.ID, cancelMsg)

		deleteMsg := tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		x.diplomat.bot.Request(deleteMsg)

	case "confirm":
		err = x.modes.DeleteMode(log, mode)
	if err != nil {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeErrorDelete"))
			x.diplomat.bot.Request(callback)
		return
	}

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeDeleted", map[string]interface{}{
			"Name": mode.Name,
			"Type": mode.Type,
		}))
		x.diplomat.bot.Request(callback)

		successMsg := x.localization.LocalizeByTd(query.Message, "MsgModeDeleted", map[string]interface{}{
		"Name": mode.Name,
		"Type": mode.Type,
	})
		x.diplomat.SendMessage(log, query.Message.Chat.ID, successMsg)

		deleteMsg := tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		x.diplomat.bot.Request(deleteMsg)
	}
}

func (x *TelegramHandler) handleModeInfoCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	if !x.rights.IsUserHasRight(log, user, "edit_mode") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(query.Message, "MsgModeModifyNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	// Extract mode type from callback data: mode_info_{modeType}
	modeType := strings.TrimPrefix(query.Data, "mode_info_")

	mode, err := x.modes.GetModeByTypeIncludingDisabled(log, modeType)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(query.Message, "MsgModeNotFound", map[string]interface{}{
			"Type": modeType,
		}))
		x.diplomat.bot.Request(callback)
		return
	}

	config := x.modes.ParseModeConfig(mode, log)
	notSet := x.localization.LocalizeBy(query.Message, "MsgModeInfoNotSet")

	gradeName := x.getGradeDisplayName(query.Message, mode.Grade)

	statusText := x.localization.LocalizeBy(query.Message, "MsgModeInfoStatusEnabled")
	if !platform.BoolValue(mode.IsEnabled, true) {
		statusText = x.localization.LocalizeBy(query.Message, "MsgModeInfoStatusDisabled")
	}

	temperature := notSet
	topP := notSet
	topK := notSet
	presencePenalty := notSet
	frequencyPenalty := notSet
	final := "false"

	if config.Params != nil {
		if config.Params.Temperature != nil {
			temperature = fmt.Sprintf("%.2f", *config.Params.Temperature)
		}
		if config.Params.TopP != nil {
			topP = fmt.Sprintf("%.2f", *config.Params.TopP)
		}
		if config.Params.TopK != nil {
			topK = fmt.Sprintf("%d", *config.Params.TopK)
		}
		if config.Params.PresencePenalty != nil {
			presencePenalty = fmt.Sprintf("%.2f", *config.Params.PresencePenalty)
		}
		if config.Params.FrequencyPenalty != nil {
			frequencyPenalty = fmt.Sprintf("%.2f", *config.Params.FrequencyPenalty)
		}
	}
	if config.Final {
		final = "true"
	}

	infoMsg := x.localization.LocalizeByTd(query.Message, "MsgModeInfo", map[string]interface{}{
		"Type":             mode.Type,
		"Name":             mode.Name,
		"Grade":            gradeName,
		"Status":           statusText,
		"Temperature":      temperature,
		"TopP":             topP,
		"TopK":             topK,
		"PresencePenalty":  presencePenalty,
		"FrequencyPenalty": frequencyPenalty,
		"Final":            final,
		"CreatedAt":        x.dateTimeFormatter.Dateify(query.Message, mode.CreatedAt),
	})

	callback := tgbotapi.NewCallback(query.ID, "")
	x.diplomat.bot.Request(callback)

	x.diplomat.SendMessage(log, query.Message.Chat.ID, x.personality.XiifyManualPlain(infoMsg))
}

func (x *TelegramHandler) getGradeDisplayName(msg *tgbotapi.Message, grade *string) string {
	if grade == nil || *grade == "" {
		return x.localization.LocalizeBy(msg, "MsgGradeAll")
	}
	switch *grade {
	case platform.GradeGold:
		return x.localization.LocalizeBy(msg, "MsgGradeGold")
	case platform.GradeSilver:
		return x.localization.LocalizeBy(msg, "MsgGradeSilver")
	case platform.GradeBronze:
		return x.localization.LocalizeBy(msg, "MsgGradeBronze")
	default:
		return x.localization.LocalizeBy(msg, "MsgGradeAll")
	}
}

func isValidModeType(modeType string) bool {
	if len(modeType) < 2 || len(modeType) > 50 {
		return false
	}
	for _, r := range modeType {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// =========================  /users command handlers  =========================

func (x *TelegramHandler) UsersCommandInfo(log *tracing.Logger, msg *tgbotapi.Message, username string) {
	user := x.retrieveUserByName(log, msg, username)
	if user == nil {
		return
	}

	status := x.localization.LocalizeBy(msg, "MsgUsersStatusActive")
	if !platform.BoolValue(user.IsActive, true) {
		status = x.localization.LocalizeBy(msg, "MsgUsersStatusBlocked")
	}

	rightsStr := x.localization.LocalizeBy(msg, "MsgUsersNoRights")
	if len(user.Rights) > 0 {
		rightsStr = strings.Join(user.Rights, ", ")
	}

	infoMsg := x.localization.LocalizeByTd(msg, "MsgUsersInfo", map[string]interface{}{
		"Username": *user.Username,
		"Status":   status,
		"Rights":   rightsStr,
	})

	var rows [][]tgbotapi.InlineKeyboardButton

	if platform.BoolValue(user.IsActive, true) {
		disableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgUsersDisableBtn"),
			"user_disable_"+*user.Username,
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(disableBtn))
	} else {
		enableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgUsersEnableBtn"),
			"user_enable_"+*user.Username,
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(enableBtn))
	}

	deleteBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersDeleteBtn"),
		"user_delete_"+*user.Username,
	)
	rightsBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersRightsBtn"),
		"user_rights_"+*user.Username,
	)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(deleteBtn, rightsBtn))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManual(msg, infoMsg), keyboard)
}

func (x *TelegramHandler) handleUserActionCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, currentUser *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, currentUser, "manage_users") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	data := query.Data

	if strings.HasPrefix(data, "user_enable_") {
		username := strings.TrimPrefix(data, "user_enable_")
		x.handleUserEnableCallback(log, query, username)
		return
	}

	if strings.HasPrefix(data, "user_disable_") {
		username := strings.TrimPrefix(data, "user_disable_")
		x.handleUserDisableCallback(log, query, username)
		return
	}

	if strings.HasPrefix(data, "user_delete_") {
		username := strings.TrimPrefix(data, "user_delete_")
		x.handleUserDeleteCallback(log, query, username)
		return
	}

	if strings.HasPrefix(data, "user_rights_") {
		username := strings.TrimPrefix(data, "user_rights_")
		x.handleUserRightsCallback(log, query, username)
		return
	}
}

func (x *TelegramHandler) handleUserEnableCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, username string) {
	msg := query.Message

	user, err := x.users.GetUserByName(log, username)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUserNotFound"))
		x.diplomat.bot.Request(callback)
		return
	}

	user.IsActive = platform.BoolPtr(true)
	_, err = x.users.UpdateUser(log, user)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersErrorEnable"))
		x.diplomat.bot.Request(callback)
		return
	}

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersEnabledCallback"))
	x.diplomat.bot.Request(callback)

	successMsg := x.localization.LocalizeByTd(msg, "MsgUsersEnabled", map[string]interface{}{
		"Username": username,
	})
	x.diplomat.SendMessage(log, msg.Chat.ID, successMsg)

	x.updateUserActionKeyboard(log, msg, user)
}

func (x *TelegramHandler) handleUserDisableCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, username string) {
	msg := query.Message

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersDisableConfirmCallback"))
	x.diplomat.bot.Request(callback)

	confirmMsg := x.localization.LocalizeByTd(msg, "MsgUsersDisableConfirm", map[string]interface{}{
		"Username": username,
	})

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersCancelBtn"),
		"user_disable_cancel_"+username,
	)
	confirmBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersConfirmDisableBtn"),
		"user_disable_confirm_"+username,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cancelBtn, confirmBtn),
	)

	x.diplomat.SendMessageWithKeyboard(log, msg.Chat.ID, x.personality.XiifyManualPlain(confirmMsg), keyboard)
}

func (x *TelegramHandler) handleUserDisableConfirmCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, currentUser *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, currentUser, "manage_users") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	data := query.Data

	if strings.HasPrefix(data, "user_disable_cancel_") {
		username := strings.TrimPrefix(data, "user_disable_cancel_")

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersDisableCancelledCallback"))
		x.diplomat.bot.Request(callback)

		cancelMsg := x.localization.LocalizeByTd(msg, "MsgUsersDisableCancelled", map[string]interface{}{
			"Username": username,
		})
		x.diplomat.SendMessage(log, msg.Chat.ID, cancelMsg)

		deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
		x.diplomat.bot.Request(deleteMsg)
		return
	}

	if strings.HasPrefix(data, "user_disable_confirm_") {
		username := strings.TrimPrefix(data, "user_disable_confirm_")

		user, err := x.users.GetUserByName(log, username)
		if err != nil {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUserNotFound"))
			x.diplomat.bot.Request(callback)
			return
		}

		user.IsActive = platform.BoolPtr(false)
		_, err = x.users.UpdateUser(log, user)
		if err != nil {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersErrorDisable"))
			x.diplomat.bot.Request(callback)
			return
		}

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersDisabledCallback"))
		x.diplomat.bot.Request(callback)

		successMsg := x.localization.LocalizeByTd(msg, "MsgUsersDisabled", map[string]interface{}{
			"Username": username,
		})
		x.diplomat.SendMessage(log, msg.Chat.ID, successMsg)

		deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
		x.diplomat.bot.Request(deleteMsg)
	}
}

func (x *TelegramHandler) handleUserDeleteCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, username string) {
	msg := query.Message

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersDeleteConfirmCallback"))
	x.diplomat.bot.Request(callback)

	confirmMsg := x.localization.LocalizeByTd(msg, "MsgUsersDeleteConfirm", map[string]interface{}{
		"Username": username,
	})

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersCancelBtn"),
		"user_delete_cancel_"+username,
	)
	confirmBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersConfirmDeleteBtn"),
		"user_delete_confirm_"+username,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cancelBtn, confirmBtn),
	)

	x.diplomat.SendMessageWithKeyboard(log, msg.Chat.ID, x.personality.XiifyManualPlain(confirmMsg), keyboard)
}

func (x *TelegramHandler) handleUserDeleteConfirmCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, currentUser *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, currentUser, "manage_users") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	data := query.Data

	if strings.HasPrefix(data, "user_delete_cancel_") {
		username := strings.TrimPrefix(data, "user_delete_cancel_")

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersDeleteCancelledCallback"))
		x.diplomat.bot.Request(callback)

		cancelMsg := x.localization.LocalizeByTd(msg, "MsgUsersDeleteCancelled", map[string]interface{}{
			"Username": username,
		})
		x.diplomat.SendMessage(log, msg.Chat.ID, cancelMsg)

		deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
		x.diplomat.bot.Request(deleteMsg)
		return
	}

	if strings.HasPrefix(data, "user_delete_confirm_") {
		username := strings.TrimPrefix(data, "user_delete_confirm_")

		err := x.users.DeleteUserByName(log, username)
		if err != nil {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersErrorRemove"))
			x.diplomat.bot.Request(callback)
			return
		}

		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersDeletedCallback"))
		x.diplomat.bot.Request(callback)

		successMsg := x.localization.LocalizeByTd(msg, "MsgUsersRemoved", map[string]interface{}{
			"Username": username,
		})
		x.diplomat.SendMessage(log, msg.Chat.ID, successMsg)

		deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
		x.diplomat.bot.Request(deleteMsg)
	}
}

func (x *TelegramHandler) handleUserRightsCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, username string) {
	msg := query.Message

	user, err := x.users.GetUserByName(log, username)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUserNotFound"))
		x.diplomat.bot.Request(callback)
		return
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	x.diplomat.bot.Request(callback)

	x.sendUserRightsMessage(log, msg.Chat.ID, user)
}

func (x *TelegramHandler) handleUserRightsToggleCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, currentUser *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, currentUser, "manage_users") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	parts := strings.SplitN(strings.TrimPrefix(query.Data, "user_right_"), "_", 2)
	if len(parts) != 2 {
		return
	}

	right := parts[0]
	username := parts[1]

	user, err := x.users.GetUserByName(log, username)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUserNotFound"))
		x.diplomat.bot.Request(callback)
		return
	}

	hasRight := false
	for _, r := range user.Rights {
		if r == right {
			hasRight = true
			break
		}
	}

	if hasRight {
		newRights := []string{}
		for _, r := range user.Rights {
			if r != right {
				newRights = append(newRights, r)
			}
		}
		user.Rights = newRights
	} else {
		user.Rights = append(user.Rights, right)
	}

	_, err = x.users.UpdateUser(log, user)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersErrorEdit"))
		x.diplomat.bot.Request(callback)
		return
	}

	action := x.localization.LocalizeBy(msg, "MsgUsersRightAdded")
	if hasRight {
		action = x.localization.LocalizeBy(msg, "MsgUsersRightRemoved")
	}

	callback := tgbotapi.NewCallback(query.ID, action)
	x.diplomat.bot.Request(callback)

	x.updateUserRightsKeyboard(log, msg, user)
}

func (x *TelegramHandler) sendUserRightsMessage(log *tracing.Logger, chatID int64, user *entities.User) {
	allRights := []struct {
		Key  string
		Desc string
	}{
		{"switch_mode", "управление режимами в чатах"},
		{"edit_mode", "редактирование режимов"},
		{"manage_users", "управление пользователями"},
		{"manage_context", "управление контекстом"},
		{"broadcast", "отправка рассылок"},
		{"manage_tariffs", "управление тарифами"},
	}

	var rightsInfo strings.Builder
	rightsInfo.WriteString("📋 **Права пользователя @" + *user.Username + "**\n\n")

	for _, r := range allRights {
		hasRight := false
		for _, ur := range user.Rights {
			if ur == r.Key {
				hasRight = true
				break
			}
		}

		icon := "❌"
		if hasRight {
			icon = "✅"
		}

		displayName := x.formatRightName(r.Key)
		rightsInfo.WriteString(fmt.Sprintf("%s **%s** — %s\n", icon, displayName, r.Desc))
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, r := range allRights {
		hasRight := false
		for _, ur := range user.Rights {
			if ur == r.Key {
				hasRight = true
				break
			}
		}

		icon := "❌"
		if hasRight {
			icon = "✅"
		}

		displayName := x.formatRightName(r.Key)
		btn := tgbotapi.NewInlineKeyboardButtonData(
			icon+" "+displayName,
			"user_right_"+r.Key+"_"+*user.Username,
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	sendMsg := tgbotapi.NewMessage(chatID, rightsInfo.String())
	sendMsg.ParseMode = "Markdown"
	sendMsg.ReplyMarkup = keyboard
	x.diplomat.bot.Send(sendMsg)
}

func (x *TelegramHandler) updateUserRightsKeyboard(log *tracing.Logger, msg *tgbotapi.Message, user *entities.User) {
	allRights := []string{"switch_mode", "edit_mode", "manage_users", "manage_context", "broadcast", "manage_tariffs"}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, r := range allRights {
		hasRight := false
		for _, ur := range user.Rights {
			if ur == r {
				hasRight = true
				break
			}
		}

		icon := "❌"
		if hasRight {
			icon = "✅"
		}

		displayName := x.formatRightName(r)
		btn := tgbotapi.NewInlineKeyboardButtonData(
			icon+" "+displayName,
			"user_right_"+r+"_"+*user.Username,
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	editMarkup := tgbotapi.NewEditMessageReplyMarkup(msg.Chat.ID, msg.MessageID, keyboard)
	x.diplomat.bot.Request(editMarkup)
}

func (x *TelegramHandler) updateUserActionKeyboard(log *tracing.Logger, msg *tgbotapi.Message, user *entities.User) {
	var rows [][]tgbotapi.InlineKeyboardButton

	if platform.BoolValue(user.IsActive, true) {
		disableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgUsersDisableBtn"),
			"user_disable_"+*user.Username,
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(disableBtn))
	} else {
		enableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgUsersEnableBtn"),
			"user_enable_"+*user.Username,
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(enableBtn))
	}

	deleteBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersDeleteBtn"),
		"user_delete_"+*user.Username,
	)
	rightsBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgUsersRightsBtn"),
		"user_rights_"+*user.Username,
	)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(deleteBtn, rightsBtn))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	editMarkup := tgbotapi.NewEditMessageReplyMarkup(msg.Chat.ID, msg.MessageID, keyboard)
	x.diplomat.bot.Request(editMarkup)
}

func (x *TelegramHandler) formatRightName(right string) string {
	parts := strings.Split(right, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(string(p[0])) + p[1:]
		}
	}
	return strings.Join(parts, " ")
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

func (x *TelegramHandler) PersonalizationCommandInfo(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	infoMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationInfo")

	addBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgPersonalizationAddBtn"),
		"personalization_add",
	)
	removeBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgPersonalizationRemoveBtn"),
		"personalization_remove",
	)
	printBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgPersonalizationPrintBtn"),
		"personalization_print",
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(addBtn, removeBtn),
		tgbotapi.NewInlineKeyboardRow(printBtn),
	)

	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManual(msg, infoMsg), keyboard)
}

func (x *TelegramHandler) handlePersonalizationCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	switch query.Data {
	case "personalization_add":
		err := x.chatState.InitPersonalizationEdit(log, msg.Chat.ID, query.From.ID)
		if err != nil {
			log.E("Failed to init personalization state", tracing.InnerError, err)
			return
		}

		callback := tgbotapi.NewCallback(query.ID, "")
		x.diplomat.bot.Request(callback)

		awaitingMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationAwaitingInput")
		x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManualPlain(awaitingMsg))

	case "personalization_remove":
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPersonalizationDeleteConfirmCallback"))
		x.diplomat.bot.Request(callback)

		confirmMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationDeleteConfirm")

		cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgPersonalizationDeleteCancelBtn"),
			"personalization_delete_cancel",
		)
		confirmBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgPersonalizationDeleteConfirmBtn"),
			"personalization_delete_confirm",
		)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(cancelBtn, confirmBtn),
		)

		x.diplomat.SendMessageWithKeyboard(log, msg.Chat.ID, x.personality.XiifyManualPlain(confirmMsg), keyboard)

	case "personalization_print":
		personalization, err := x.personalizations.GetPersonalizationByUser(log, user)
		if err != nil {
			if errors.Is(err, repository.ErrPersonalizationNotFound) {
				callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPersonalizationNotFound"))
				x.diplomat.bot.Request(callback)
				return
			}

			log.E("Failed to get personalization", tracing.InnerError, err)
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPersonalizationErrorPrint"))
			x.diplomat.bot.Request(callback)
			return
		}

		callback := tgbotapi.NewCallback(query.ID, "")
		x.diplomat.bot.Request(callback)

		response := x.localization.LocalizeByTd(msg, "MsgPersonalizationPrint", map[string]interface{}{
			"Info": personalization.Prompt,
		})
		x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, response))
	}
}

func (x *TelegramHandler) handlePersonalizationDeleteCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if query.Data == "personalization_delete_cancel" {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPersonalizationDeleteCancelledCallback"))
		x.diplomat.bot.Request(callback)

		cancelMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationDeleteCancelled")
		x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, cancelMsg))

		deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
		x.diplomat.bot.Request(deleteMsg)
		return
	}

	err := x.personalizations.DeletePersonalization(log, user)
	if err != nil {
		if errors.Is(err, repository.ErrPersonalizationNotFound) {
			callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPersonalizationNotFound"))
			x.diplomat.bot.Request(callback)

			deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
			x.diplomat.bot.Request(deleteMsg)
			return
		}

		log.E("Failed to delete personalization", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPersonalizationErrorRemove"))
		x.diplomat.bot.Request(callback)
		return
	}

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPersonalizationDeletedCallback"))
	x.diplomat.bot.Request(callback)

	successMsg := x.localization.LocalizeBy(msg, "MsgPersonalizationRemoved")
	x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, successMsg))

	deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
	x.diplomat.bot.Request(deleteMsg)
}

func (x *TelegramHandler) handlePersonalizationInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	prompt := strings.TrimSpace(msg.Text)

	x.chatState.ClearState(log, msg.Chat.ID, msg.From.ID)

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

	canManage := msg.Chat.Type == "private" || x.rights.IsUserHasRight(log, user, "manage_context")

	if !canManage {
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, infoMsg))
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	clearBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgContextClearBtn"),
		"context_clear",
	)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(clearBtn))

	if stats.Enabled {
		disableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgContextDisableBtn"),
			"context_disable",
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(disableBtn))
	} else {
		enableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgContextEnableBtn"),
			"context_enable",
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(enableBtn))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManual(msg, infoMsg), keyboard)
}

func (x *TelegramHandler) handleContextToggleCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if msg.Chat.Type != "private" && !x.rights.IsUserHasRight(log, user, "manage_context") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgContextNoAccess"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	var enable bool
	var successMsgKey string
	var errorMsgKey string
	var callbackMsgKey string

	if query.Data == "context_enable" {
		enable = true
		successMsgKey = "MsgContextEnabled"
		errorMsgKey = "MsgContextEnableError"
		callbackMsgKey = "MsgContextEnabledCallback"
	} else {
		enable = false
		successMsgKey = "MsgContextDisabled"
		errorMsgKey = "MsgContextDisableError"
		callbackMsgKey = "MsgContextDisabledCallback"
	}

	err := x.contextManager.SetEnabled(log, platform.ChatID(msg.Chat.ID), enable)
	if err != nil {
		log.E("Failed to toggle context", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, errorMsgKey))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, callbackMsgKey))
	if _, err := x.diplomat.bot.Request(callback); err != nil {
		log.E("Failed to answer callback", tracing.InnerError, err)
	}

	successMsg := x.localization.LocalizeBy(msg, successMsgKey)
	x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, successMsg))

	var rows [][]tgbotapi.InlineKeyboardButton

	clearBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgContextClearBtn"),
		"context_clear",
	)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(clearBtn))

	if enable {
		disableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgContextDisableBtn"),
			"context_disable",
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(disableBtn))
	} else {
		enableBtn := tgbotapi.NewInlineKeyboardButtonData(
			x.localization.LocalizeBy(msg, "MsgContextEnableBtn"),
			"context_enable",
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(enableBtn))
	}

	newKeyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	editMsg := tgbotapi.NewEditMessageReplyMarkup(msg.Chat.ID, msg.MessageID, newKeyboard)
	if _, err := x.diplomat.bot.Request(editMsg); err != nil {
		log.E("Failed to edit message keyboard", tracing.InnerError, err)
	}
}

func (x *TelegramHandler) handleContextClearCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if msg.Chat.Type != "private" && !x.rights.IsUserHasRight(log, user, "manage_context") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgContextNoAccess"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgContextClearConfirmCallback"))
	if _, err := x.diplomat.bot.Request(callback); err != nil {
		log.E("Failed to answer callback", tracing.InnerError, err)
	}

	confirmMsg := x.localization.LocalizeBy(msg, "MsgContextClearConfirm")

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgContextClearCancelBtn"),
		"context_clear_cancel",
	)
	confirmBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgContextClearConfirmBtn"),
		"context_clear_confirm",
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cancelBtn, confirmBtn),
	)

	x.diplomat.SendMessageWithKeyboard(log, msg.Chat.ID, x.personality.XiifyManual(msg, confirmMsg), keyboard)
}

func (x *TelegramHandler) handleContextClearConfirmCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if msg.Chat.Type != "private" && !x.rights.IsUserHasRight(log, user, "manage_context") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgContextNoAccess"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	if query.Data == "context_clear_cancel" {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgContextClearCancelledCallback"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}

		cancelMsg := x.localization.LocalizeBy(msg, "MsgContextClearCancelled")
		x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, cancelMsg))

		deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
		if _, err := x.diplomat.bot.Request(deleteMsg); err != nil {
			log.E("Failed to delete confirmation message", tracing.InnerError, err)
		}
		return
	}

	err := x.contextManager.Clear(log, platform.ChatID(msg.Chat.ID))
	if err != nil {
		log.E("Failed to clear context", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgContextRefreshError"))
		if _, err := x.diplomat.bot.Request(callback); err != nil {
			log.E("Failed to answer callback", tracing.InnerError, err)
		}
		return
	}

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgContextClearedCallback"))
	if _, err := x.diplomat.bot.Request(callback); err != nil {
		log.E("Failed to answer callback", tracing.InnerError, err)
	}

	successMsg := x.localization.LocalizeBy(msg, "MsgContextRefreshed")
	x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, successMsg))

	deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
	if _, err := x.diplomat.bot.Request(deleteMsg); err != nil {
		log.E("Failed to delete confirmation message", tracing.InnerError, err)
	}
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

func (x *TelegramHandler) PardonCommandShowList(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Pardon command show list completed", "telegram.command.pardon.show.list", "chat_id", msg.Chat.ID)()

	const maxBannedUsers = 99

	bans, err := x.bans.GetAllActiveBans(log, maxBannedUsers)
	if err != nil {
		log.E("Failed to get active bans", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgPardonErrorList")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	if len(bans) == 0 {
		noBansMsg := x.localization.LocalizeBy(msg, "MsgPardonNoBans")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, noBansMsg))
		return
	}

	message := x.localization.LocalizeBy(msg, "MsgPardonListTitle")

	displayCount := len(bans)
	if displayCount > 33 {
		displayCount = 33
	}

	for i := 0; i < displayCount; i++ {
		ban := bans[i]
		duration, _ := x.bans.ParseDuration(ban.Duration)
		expiresAt := ban.BannedAt.Add(duration)
		remaining := x.bans.GetRemainingDuration(expiresAt)
		remainingFormatted := x.bans.FormatRemainingTime(msg, remaining)

		username := "unknown"
		fullname := "Unknown"
		if ban.User.Username != nil {
			username = *ban.User.Username
		}
		if ban.User.Fullname != nil {
			fullname = *ban.User.Fullname
		}

		itemData := map[string]interface{}{
			"Fullname":  fullname,
			"Username":  username,
			"Remaining": remainingFormatted,
		}
		message += x.localization.LocalizeByTd(msg, "MsgPardonListItem", itemData)
	}

	message += x.localization.LocalizeBy(msg, "MsgPardonListFooter")

	var buttons [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, ban := range bans {
		username := "?"
		if ban.User.Username != nil {
			username = "@" + *ban.User.Username
		} else if ban.User.Fullname != nil {
			fullname := *ban.User.Fullname
			if len([]rune(fullname)) > 15 {
				username = string([]rune(fullname)[:15]) + "…"
			} else {
				username = fullname
			}
		}

		btn := tgbotapi.NewInlineKeyboardButtonData(username, "pardon_user_"+ban.UserID.String())
		row = append(row, btn)

		if len(row) == 3 || i == len(bans)-1 {
			buttons = append(buttons, row)
			row = nil
		}
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManual(msg, message), keyboard)
}

func (x *TelegramHandler) PardonCommandApply(log *tracing.Logger, msg *tgbotapi.Message, userID uuid.UUID) {
	err := x.bans.DeleteBansByUser(log, userID)
	if err != nil {
		log.E("Failed to pardon user", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgBanErrorRemove")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	user, err := x.users.GetUserByID(log, userID)
	if err != nil {
		log.W("Failed to get user after pardon", tracing.InnerError, err)
		return
	}

	displayName := "unknown"
	if user.Username != nil {
		displayName = *user.Username
	}

	successMsg := x.localization.LocalizeByTd(msg, "MsgBanPardon", map[string]interface{}{
		"Username": displayName,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

// =========================  /health command handlers  =========================

func (x *TelegramHandler) HealthCommand(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Health command completed", "telegram.command.health", "chat_id", msg.Chat.ID)()

	statusOk := x.localization.LocalizeBy(msg, "MsgHealthStatusOk")
	statusFail := x.localization.LocalizeBy(msg, "MsgHealthStatusFail")

	// Database check
	dbStatus := statusOk
	if err := x.health.CheckDatabaseHealth(log); err != nil {
		dbStatus = statusFail
	}

	// Redis check
	redisStatus := statusOk
	if err := x.health.CheckRedisHealth(log); err != nil {
		redisStatus = statusFail
	}

	// Proxy check
	proxyStatus := statusOk
	if err := x.health.CheckProxyHealth(log); err != nil {
		proxyStatus = statusFail
	}

	// OpenRouter API check
	openrouterStatus := statusOk
	if err := x.health.CheckOpenRouterHealth(log); err != nil {
		openrouterStatus = statusFail
	}

	// Unleash check
	unleashStatus := statusOk
	if err := x.health.CheckUnleashHealth(log); err != nil {
		unleashStatus = statusFail
	}

	// Telegram API check
	telegramStatus := statusOk
	if err := x.health.CheckTelegramHealth(log, x.diplomat.bot); err != nil {
		telegramStatus = statusFail
	}

	// System status (overall)
	systemStatus := statusOk
	if dbStatus == statusFail || redisStatus == statusFail {
		systemStatus = statusFail
	}

	// Runtime metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryMB := float64(memStats.Alloc) / 1000 / 1000
	goroutines := runtime.NumGoroutine()
	goVersion := runtime.Version()

	uptime := time.Since(platform.GetAppStartTime())
	uptimeFormatted := x.dateTimeFormatter.Uptimeify(msg, uptime)

	version := platform.GetAppVersion()
	buildTime := platform.GetAppBuildTime()
	buildTimeFormatted := x.dateTimeFormatter.FormatBuildTime(msg, buildTime)

	title := x.localization.LocalizeBy(msg, "MsgHealthTitle")

	dbMsg := x.localization.LocalizeByTd(msg, "MsgHealthDatabase", map[string]interface{}{
		"Status": dbStatus,
	})

	redisMsg := x.localization.LocalizeByTd(msg, "MsgHealthRedis", map[string]interface{}{
		"Status": redisStatus,
	})

	proxyMsg := x.localization.LocalizeByTd(msg, "MsgHealthProxy", map[string]interface{}{
		"Status": proxyStatus,
	})

	openrouterMsg := x.localization.LocalizeByTd(msg, "MsgHealthOpenRouter", map[string]interface{}{
		"Status": openrouterStatus,
	})

	unleashMsg := x.localization.LocalizeByTd(msg, "MsgHealthUnleash", map[string]interface{}{
		"Status": unleashStatus,
	})

	telegramMsg := x.localization.LocalizeByTd(msg, "MsgHealthTelegram", map[string]interface{}{
		"Status": telegramStatus,
	})

	systemMsg := x.localization.LocalizeByTd(msg, "MsgHealthSystem", map[string]interface{}{
		"Status": systemStatus,
	})

	uptimeMsg := x.localization.LocalizeByTd(msg, "MsgHealthUptime", map[string]interface{}{
		"Uptime": uptimeFormatted,
	})

	memoryMsg := x.localization.LocalizeByTd(msg, "MsgHealthMemory", map[string]interface{}{
		"Memory": fmt.Sprintf("%.2f", memoryMB),
	})

	goroutinesMsg := x.localization.LocalizeByTd(msg, "MsgHealthGoroutines", map[string]interface{}{
		"Count": goroutines,
	})

	goVersionMsg := x.localization.LocalizeByTd(msg, "MsgHealthGoVersion", map[string]interface{}{
		"Version": goVersion,
	})

	versionMsg := x.localization.LocalizeByTd(msg, "MsgHealthVersion", map[string]interface{}{
		"Version": version,
	})

	buildTimeMsg := x.localization.LocalizeByTd(msg, "MsgHealthBuildTime", map[string]interface{}{
		"BuildTime": buildTimeFormatted,
	})

	response := title + dbMsg + redisMsg + proxyMsg + openrouterMsg + unleashMsg + telegramMsg + systemMsg + uptimeMsg + memoryMsg + goroutinesMsg + goVersionMsg + versionMsg + buildTimeMsg
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

// =========================  /pardon callback handlers  =========================

func (x *TelegramHandler) handlePardonCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, user, "manage_users") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUsersNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	userIDStr := strings.TrimPrefix(query.Data, "pardon_user_")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.E("Failed to parse user ID from callback", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgPardonError"))
		x.diplomat.bot.Request(callback)
		return
	}

	targetUser, err := x.users.GetUserByID(log, userID)
	if err != nil {
		log.E("Failed to get user", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgUserNotFound"))
		x.diplomat.bot.Request(callback)
		return
	}

	err = x.bans.DeleteBansByUser(log, userID)
	if err != nil {
		log.E("Failed to pardon user", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgBanErrorRemove"))
		x.diplomat.bot.Request(callback)
		return
	}

	displayName := "unknown"
	if targetUser.Username != nil {
		displayName = *targetUser.Username
	}

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(msg, "MsgPardonCallback", map[string]interface{}{
		"Username": displayName,
	}))
	x.diplomat.bot.Request(callback)

	successMsg := x.localization.LocalizeByTd(msg, "MsgBanPardon", map[string]interface{}{
		"Username": displayName,
	})
	x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, successMsg))

	const maxBannedUsers = 99
	remainingBans, err := x.bans.GetAllActiveBans(log, maxBannedUsers)
	if err != nil {
		log.E("Failed to get remaining bans", tracing.InnerError, err)
		return
	}

	if len(remainingBans) == 0 {
		editMarkup := tgbotapi.NewEditMessageReplyMarkup(msg.Chat.ID, msg.MessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
		x.diplomat.bot.Request(editMarkup)
		return
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, ban := range remainingBans {
		username := "?"
		if ban.User.Username != nil {
			username = "@" + *ban.User.Username
		} else if ban.User.Fullname != nil {
			fullname := *ban.User.Fullname
			if len([]rune(fullname)) > 15 {
				username = string([]rune(fullname)[:15]) + "…"
			} else {
				username = fullname
			}
		}

		btn := tgbotapi.NewInlineKeyboardButtonData(username, "pardon_user_"+ban.UserID.String())
		row = append(row, btn)

		if len(row) == 3 || i == len(remainingBans)-1 {
			buttons = append(buttons, row)
			row = nil
		}
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	editMarkup := tgbotapi.NewEditMessageReplyMarkup(msg.Chat.ID, msg.MessageID, keyboard)
	if _, err := x.diplomat.bot.Request(editMarkup); err != nil {
		log.W("Failed to update pardon keyboard", tracing.InnerError, err)
	}
}

// =========================  /broadcast command handlers  =========================

// =========================  /tariff command handlers  =========================

func (x *TelegramHandler) TariffCommandShowList(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Tariff command show list completed", "telegram.command.tariff.show.list", "chat_id", msg.Chat.ID)()

	tariffs, err := x.tariffs.GetAllLatest(log)
	if err != nil {
		log.E("Failed to get tariffs list", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgTariffErrorList")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	message := x.localization.LocalizeBy(msg, "MsgTariffListTitle")

	if len(tariffs) > 0 {
		for _, t := range tariffs {
			itemMsg := x.localization.LocalizeByTd(msg, "MsgTariffListItem", map[string]interface{}{
				"Key":         t.Key,
				"DisplayName": t.DisplayName,
			})
			message += itemMsg
		}
	} else {
		message += x.localization.LocalizeBy(msg, "MsgTariffNoTariffsInList")
	}

	message += x.localization.LocalizeBy(msg, "MsgTariffListFooter")

	var buttons [][]tgbotapi.InlineKeyboardButton

	addBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgTariffAddBtn"),
		"tariff_add",
	)
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(addBtn))

	var row []tgbotapi.InlineKeyboardButton
	for i, t := range tariffs {
		btn := tgbotapi.NewInlineKeyboardButtonData("🔍 "+t.DisplayName, "tariff_info_"+t.Key)
		row = append(row, btn)

		if len(row) == 3 || i == len(tariffs)-1 {
			buttons = append(buttons, row)
			row = nil
		}
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManual(msg, message), keyboard)
}

func (x *TelegramHandler) TariffCommandCreateStart(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	defer tracing.ProfilePoint(log, "Tariff command create start completed", "telegram.command.tariff.create.start", "chat_id", msg.Chat.ID)()

	err := x.chatState.InitTariffCreation(log, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.E("Failed to init tariff creation state", tracing.InnerError, err)
		errorMsg := x.localization.LocalizeBy(msg, "MsgTariffErrorCreate")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	startMsg := x.localization.LocalizeBy(msg, "MsgTariffCreateStart")
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, startMsg))
}

func (x *TelegramHandler) handleTariffKeyInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	tariffKey := strings.TrimSpace(strings.ToLower(msg.Text))

	if !isValidTariffKey(tariffKey) {
		errorMsg := x.localization.LocalizeBy(msg, "MsgTariffCreateInvalidKey")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	err := x.chatState.SetTariffKey(log, msg.Chat.ID, msg.From.ID, tariffKey)
	if err != nil {
		log.E("Failed to set tariff key in state", tracing.InnerError, err)
		return
	}

	nextMsg := x.localization.LocalizeByTd(msg, "MsgTariffCreateAwaitingConfig", map[string]interface{}{
		"Key": tariffKey,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, nextMsg))
}

func (x *TelegramHandler) handleTariffConfigInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message, state *repository.ChatStateData) {
	configJSON := strings.TrimSpace(msg.Text)

	var config repository.TariffConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		errorMsg := x.localization.LocalizeBy(msg, "MsgTariffErrorConfigParse")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	tariff, err := x.tariffs.CreateTariff(log, state.TariffKey, &config)
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
		case errors.Is(err, repository.ErrTariffInvalidLimit):
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorInvalidLimit")
		default:
			errorMsg = x.localization.LocalizeBy(msg, "MsgTariffErrorCreate")
		}
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	x.chatState.ClearState(log, msg.Chat.ID, msg.From.ID)

	successMsg := x.localization.LocalizeByTd(msg, "MsgTariffAdded", map[string]interface{}{
		"Key":         tariff.Key,
		"DisplayName": tariff.DisplayName,
		"ID":          tariff.ID,
	})
	x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, successMsg))
}

func isValidTariffKey(key string) bool {
	if len(key) < 2 || len(key) > 50 {
		return false
	}
	for _, r := range key {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func (x *TelegramHandler) handleTariffAddCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, user, "manage_tariffs") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgTariffNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	err := x.chatState.InitTariffCreation(log, msg.Chat.ID, query.From.ID)
	if err != nil {
		log.E("Failed to init tariff creation state", tracing.InnerError, err)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgTariffErrorCreate"))
		x.diplomat.bot.Request(callback)
		return
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	x.diplomat.bot.Request(callback)

	startMsg := x.localization.LocalizeBy(msg, "MsgTariffCreateStart")
	x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManualPlain(startMsg))
}

func (x *TelegramHandler) handleTariffInfoCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, user, "manage_tariffs") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgTariffNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	tariffKey := strings.TrimPrefix(query.Data, "tariff_info_")

	tariff, err := x.tariffs.GetLatestByKey(log, tariffKey)
	if err != nil {
		log.E("Failed to get tariff", tracing.InnerError, err, "key", tariffKey)
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeByTd(msg, "MsgTariffNotFound", map[string]interface{}{
			"Key": tariffKey,
		}))
		x.diplomat.bot.Request(callback)
		return
	}

	priceStr := fmt.Sprintf("%d₽", tariff.Price)

	data := map[string]interface{}{
		"ID":                   tariff.ID,
		"Key":                  tariff.Key,
		"DisplayName":          tariff.DisplayName,
		"RequestsPerDay":       tariff.RequestsPerDay,
		"RequestsPerMonth":     tariff.RequestsPerMonth,
		"TokensPerDay":         tariff.TokensPerDay,
		"TokensPerMonth":       tariff.TokensPerMonth,
		"SpendingDailyLimit":   tariff.SpendingDailyLimit.String(),
		"SpendingMonthlyLimit": tariff.SpendingMonthlyLimit.String(),
		"Price":                priceStr,
		"CreatedAt":            tariff.CreatedAt.Format("02.01.2006 15:04:05"),
	}

	infoMsg := x.localization.LocalizeByTd(msg, "MsgTariffInfo", data)

	callback := tgbotapi.NewCallback(query.ID, "")
	x.diplomat.bot.Request(callback)

	x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManualPlain(infoMsg))
}

func (x *TelegramHandler) BroadcastCommandStart(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	err := x.chatState.InitBroadcast(log, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.E("Failed to init broadcast state", tracing.InnerError, err)
		return
	}

	awaitingMsg := x.localization.LocalizeBy(msg, "MsgBroadcastAwaitingText")
	x.diplomat.Reply(log, msg, x.personality.XiifyManualPlain(awaitingMsg))
}

func (x *TelegramHandler) handleBroadcastInput(log *tracing.Logger, user *entities.User, msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)

	if len(text) < 10 {
		errorMsg := x.localization.LocalizeBy(msg, "MsgBroadcastTextTooShort")
		x.diplomat.Reply(log, msg, x.personality.XiifyManual(msg, errorMsg))
		return
	}

	err := x.chatState.SetBroadcastText(log, msg.Chat.ID, msg.From.ID, text)
	if err != nil {
		log.E("Failed to set broadcast text in state", tracing.InnerError, err)
		return
	}

	confirmMsg := x.localization.LocalizeByTd(msg, "MsgBroadcastConfirm", map[string]interface{}{
		"Text": text,
	})

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgBroadcastCancelBtn"),
		"broadcast_cancel",
	)
	sendBtn := tgbotapi.NewInlineKeyboardButtonData(
		x.localization.LocalizeBy(msg, "MsgBroadcastSendBtn"),
		"broadcast_send",
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(cancelBtn, sendBtn),
	)

	x.diplomat.ReplyWithKeyboard(log, msg, x.personality.XiifyManualPlain(confirmMsg), keyboard)
}

func (x *TelegramHandler) handleBroadcastConfirmCallback(log *tracing.Logger, query *tgbotapi.CallbackQuery, user *entities.User) {
	msg := query.Message

	if !x.rights.IsUserHasRight(log, user, "broadcast") {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgBroadcastNoAccess"))
		x.diplomat.bot.Request(callback)
		return
	}

	state, err := x.chatState.GetState(log, msg.Chat.ID, query.From.ID)
	if err != nil || state == nil || state.Status != repository.ChatStateConfirmBroadcast {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgBroadcastErrorCreate"))
		x.diplomat.bot.Request(callback)
		return
	}

	x.chatState.ClearState(log, msg.Chat.ID, query.From.ID)

	if query.Data == "broadcast_cancel" {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgBroadcastCancelledCallback"))
		x.diplomat.bot.Request(callback)

		cancelMsg := x.localization.LocalizeBy(msg, "MsgBroadcastCancelled")
		x.diplomat.SendMessage(log, msg.Chat.ID, x.personality.XiifyManual(msg, cancelMsg))

		deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
		x.diplomat.bot.Request(deleteMsg)
		return
	}

	text := state.BroadcastText

	_, err = x.broadcast.CreateBroadcast(log, user.ID, text)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgBroadcastErrorCreate"))
		x.diplomat.bot.Request(callback)
		return
	}

	chatIDs, err := x.messages.GetAllChatIDs(log)
	if err != nil {
		callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgBroadcastErrorGetChats"))
		x.diplomat.bot.Request(callback)
		return
	}

	callback := tgbotapi.NewCallback(query.ID, x.localization.LocalizeBy(msg, "MsgBroadcastSendingCallback"))
	x.diplomat.bot.Request(callback)

	deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
	x.diplomat.bot.Request(deleteMsg)

	x.diplomat.SendMessage(log, msg.Chat.ID, x.localization.LocalizeBy(msg, "MsgBroadcastStarted"))

	successCount := 0
	failCount := 0

	for _, chatID := range chatIDs {
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

		time.Sleep(100 * time.Millisecond)
	}

	resultMsg := x.localization.LocalizeByTd(msg, "MsgBroadcastFinished", map[string]interface{}{
		"Success": successCount,
		"Fail":    failCount,
		"Total":   len(chatIDs),
	})
	x.diplomat.SendMessage(log, msg.Chat.ID, resultMsg)
}
