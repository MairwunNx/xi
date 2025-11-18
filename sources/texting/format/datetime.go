package format

import (
	"fmt"
	"time"
)

// todo: localize.
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