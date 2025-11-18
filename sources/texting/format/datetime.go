package format

import (
	"time"
	"ximanager/sources/localization"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type DateTimeFormatter struct {
	localization *localization.LocalizationManager
}

func NewDateTimeFormatter(localization *localization.LocalizationManager) *DateTimeFormatter {
	return &DateTimeFormatter{
		localization: localization,
	}
}

func (f *DateTimeFormatter) Ageify(msg *tgbotapi.Message, createdAt time.Time) string {
	age := time.Since(createdAt)
	
	days := int(age.Hours() / 24)
	
	if days == 0 {
		return f.localization.LocalizeBy(msg, "AgeifyToday")
	}
	
	if days < 7 {
		return f.localization.LocalizeByTd(msg, "AgeifyDaysAgo", map[string]interface{}{"Count": days})
	}
	
	weeks := days / 7
	if weeks < 5 {
		return f.localization.LocalizeByTd(msg, "AgeifyWeeksAgo", map[string]interface{}{"Count": weeks})
	}
	
	months := days / 30
	if months < 12 {
		return f.localization.LocalizeByTd(msg, "AgeifyMonthsAgo", map[string]interface{}{"Count": months})
	}
	
	years := days / 365
	return f.localization.LocalizeByTd(msg, "AgeifyYearsAgo", map[string]interface{}{"Count": years})
}