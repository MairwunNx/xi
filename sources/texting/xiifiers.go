package texting

import "fmt"

func Xiify(text string) string {
	return fmt.Sprintf("%s%s", MsgXiResponse, text)
}

func XiifyManual(text string) string {
	return fmt.Sprintf("%s%s", MsgXiManualResponse, text)
}

func XiifyAudio(text string) string {
	return fmt.Sprintf("%s%s", MsgAudioSuccess, text)
}
