package format

import (
	"fmt"

	"github.com/shopspring/decimal"
)

func Currencify(value float64) string {
	return fmt.Sprintf("$%0.2f", value)
}

func CurrencifyDecimal(value decimal.Decimal) string {
	return Currencify(value.InexactFloat64())
}