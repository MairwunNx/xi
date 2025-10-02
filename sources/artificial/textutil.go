package artificial

import (
	"fmt"
	"time"
	"ximanager/sources/texting"
)

func UserReq(persona string, req string) string {
	moscowTime := time.Now().UTC().Add(3 * time.Hour)
	timestamp := moscowTime.Format("Monday, 02 January 2006, 15:04:05")
	
	systemData := fmt.Sprintf("System data:\nDate and time: %s\n\n", timestamp)
	
	return systemData + fmt.Sprintf(texting.InternalAIGreetingMessage, persona, req)
}