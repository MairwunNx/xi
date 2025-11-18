package format

import (
	"github.com/shopspring/decimal"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

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