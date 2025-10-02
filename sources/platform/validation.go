package platform

import (
	"fmt"
	"regexp"
)

var (
	OpenAITokenPattern = regexp.MustCompile(`^sk-[A-Za-z0-9]{48}$`)
	TelegramBotTokenPattern = regexp.MustCompile(`^[0-9]+:AA[0-9A-Za-z\-_]{33}$`)
	Base64Pattern = regexp.MustCompile(`^(eyJ|YTo|Tzo|PD[89]|aHR0cHM6L|aHR0cDo|rO0)[a-zA-Z0-9+/]+={0,2}$`)
)

func ValidateOpenAIToken(token string) error {
	if token == "" {
		return fmt.Errorf("OpenAI API token is required")
	}
	
	if !OpenAITokenPattern.MatchString(token) {
		return fmt.Errorf("invalid OpenAI API token format: expected sk-[A-Za-z0-9]{48}")
	}
	
	return nil
}

func ValidateTelegramBotToken(token string) error {
	if token == "" {
		return fmt.Errorf("Telegram Bot API token is required")
	}
	
	if !TelegramBotTokenPattern.MatchString(token) {
		return fmt.Errorf("invalid Telegram Bot API token format: expected [0-9]+:AA[0-9A-Za-z\\-_]{33}")
	}
	
	return nil
}

func ValidateBase64(value string, fieldName string) error {
	if value == "" {
		return nil
	}
	
	if !Base64Pattern.MatchString(value) {
		return fmt.Errorf("invalid base64 format for %s", fieldName)
	}
	
	return nil
}

func ValidateNotEmpty(value string, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}
