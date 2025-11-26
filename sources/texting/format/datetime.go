package format

import (
	"strings"
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

func (f *DateTimeFormatter) Uptimeify(msg *tgbotapi.Message, duration time.Duration) string {
	if duration <= 0 {
		return f.localization.LocalizeBy(msg, "UptimeJustStarted")
	}

	totalMinutes := int(duration.Minutes())
	totalHours := int(duration.Hours())
	totalDays := totalHours / 24
	
	years := totalDays / 365
	remainingDays := totalDays % 365
	months := remainingDays / 30
	days := remainingDays % 30
	hours := totalHours % 24
	minutes := totalMinutes % 60

	var parts []string

	if years > 0 {
		yearsStr := f.localization.LocalizeByTd(msg, "UptimeYears", map[string]interface{}{"Count": years})
		parts = append(parts, yearsStr)
	}
	if months > 0 {
		monthsStr := f.localization.LocalizeByTd(msg, "UptimeMonths", map[string]interface{}{"Count": months})
		parts = append(parts, monthsStr)
	}
	if days > 0 {
		daysStr := f.localization.LocalizeByTd(msg, "UptimeDays", map[string]interface{}{"Count": days})
		parts = append(parts, daysStr)
	}
	if hours > 0 {
		hoursStr := f.localization.LocalizeByTd(msg, "UptimeHours", map[string]interface{}{"Count": hours})
		parts = append(parts, hoursStr)
	}
	if minutes > 0 || len(parts) == 0 {
		minutesStr := f.localization.LocalizeByTd(msg, "UptimeMinutes", map[string]interface{}{"Count": minutes})
		parts = append(parts, minutesStr)
	}

	return strings.Join(parts, " ")
}

func (f *DateTimeFormatter) FormatBuildTime(msg *tgbotapi.Message, buildTime string) string {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.UnixDate,
	}

	var parsed time.Time
	var err error
	for _, format := range formats {
		parsed, err = time.Parse(format, buildTime)
		if err == nil {
			break
		}
	}

	if err != nil {
		return buildTime
	}

	formatStr := f.localization.LocalizeBy(msg, "BuildTimeFormat")
	return parsed.Format(formatStr)
}

func (f *DateTimeFormatter) Dateify(msg *tgbotapi.Message, t time.Time) string {
	formatStr := f.localization.LocalizeBy(msg, "BuildTimeFormat")
	return t.Format(formatStr)
}