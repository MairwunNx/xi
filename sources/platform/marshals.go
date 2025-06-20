package platform

import (
	"strconv"
	"strings"
)

func (c *ChatID) UnmarshalText(text []byte) error {
	str := string(text)
	if strings.HasPrefix(str, "~") {
		str = "-" + strings.TrimPrefix(str, "~")
	}
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}
	*c = ChatID(val)
	return nil
}
