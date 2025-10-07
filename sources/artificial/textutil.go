package artificial

import (
	"fmt"
	"time"
)

func UserReq(persona string, req string) string {
	localNow := time.Now().In(time.Local)
	timestamp := localNow.Format("Monday, 02 January 2006, 15:04:05")

	return fmt.Sprintf("System data:\nDate and time: %s\nParticipant: '%s'\n\nMessage:\n%s", timestamp, persona, req)
}