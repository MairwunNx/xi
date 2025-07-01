package artificial

import (
	"fmt"
	"ximanager/sources/texting"
)

func UserReq(persona string, req string) string {
	return fmt.Sprintf(texting.InternalAIGreetingMessage, persona, req)
}