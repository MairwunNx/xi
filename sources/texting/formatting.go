package texting

import (
	"fmt"
	"time"
	"unicode"

	"github.com/shopspring/decimal"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func Currencify(value float64) string {
	return fmt.Sprintf("$%0.2f", value)
}
func CurrencifyDecimal(value decimal.Decimal) string {
	return Currencify(value.InexactFloat64())
}

var ru = message.NewPrinter(language.Russian)

func Numberify(value int64) string {
	return ru.Sprintf("%d", value)
}

func Decimalify(value decimal.Decimal) string {
	return ru.Sprintf("%.2f", value.InexactFloat64())
}

func DecimalifyFloat(value float64) string {
	return ru.Sprintf("%.2f", value)
}

func Pluralify(count int, one, few, many string) string {
	n := count % 100
	if n >= 11 && n <= 19 {
		return many
	}
	n = count % 10
	if n == 1 {
		return one
	}
	if n >= 2 && n <= 4 {
		return few
	}
	return many
}

func Ageify(createdAt time.Time) string {
	age := time.Since(createdAt)
	
	days := int(age.Hours() / 24)
	
	if days == 0 {
		return "сегодня"
	}
	
	if days < 7 {
		return fmt.Sprintf("%d %s назад", days, Pluralify(days, "день", "дня", "дней"))
	}
	
	weeks := days / 7
	if weeks < 5 {
		return fmt.Sprintf("%d %s назад", weeks, Pluralify(weeks, "неделю", "недели", "недель"))
	}
	
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%d %s назад", months, Pluralify(months, "месяц", "месяца", "месяцев"))
	}
	
	years := days / 365
	return fmt.Sprintf("%d %s назад", years, Pluralify(years, "год", "года", "лет"))
}

func SmartTruncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	truncated := text[:maxLen]

	for i := len(truncated) - 1; i >= 0; i-- {
		if unicode.IsSpace(rune(truncated[i])) {
			return truncated[:i]
		}
	}

	return truncated
}