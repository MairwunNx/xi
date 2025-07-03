package texting

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/shopspring/decimal"
)

func Currencify(value float64) string {
	return fmt.Sprintf("$%s", humanize.CommafWithDigits(value, 2))
}

func CurrencifyDecimal(value decimal.Decimal) string {
	return Currencify(value.InexactFloat64())
}

func Numberify(value int64) string {
	return humanize.Comma(value)
}

func Decimalify(value decimal.Decimal) string {
	return humanize.CommafWithDigits(value.InexactFloat64(), 2)
}

func DecimalifyFloat(value float64) string {
	return humanize.CommafWithDigits(value, 2)
} 