package texting

/*
#cgo pkg-config: python3-embed
#include "../../markdownify.c"
#include <stdlib.h>
*/
import "C"
import (
	"log/slog"
	"strings"
	"sync"
	"unsafe"
)

var (
	markdownMutex sync.Mutex
)

func EscapeMarkdown(input string) string {
	if len(input) == 0 {
		return input
	}

	markdownMutex.Lock()
	defer markdownMutex.Unlock()

	cInput := C.CString(input)
	defer C.free(unsafe.Pointer(cInput))

	cResult := C.markdownify(cInput)
	if cResult == nil {
		slog.Error("markdownify returned null, using fallback")
		return escapeBasic(input)
	}
	defer C.free_result(cResult)

	result := C.GoString(cResult)
	
	if len(result) == 0 {
		slog.Warn("markdownify returned empty result, using fallback")
		return escapeBasic(input)
	}

	return result
}

func escapeBasic(input string) string {
	const escapable = "_*[]()~`>#+-=|{}.!"
	
	var result strings.Builder
	result.Grow(len(input) * 2)
	
	for _, char := range input {
		if strings.ContainsRune(escapable, char) {
			result.WriteRune('\\')
		}
		result.WriteRune(char)
	}
	
	return result.String()
}
