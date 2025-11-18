package texting

/*
#cgo pkg-config: python3-embed
#include "../../markdownify.c"
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"
)

type mdReq struct {
	in   string
	out  chan string
}

var (
	mdOnce sync.Once
	mdCh   chan mdReq
)

func ensureMDWorker() {
	mdOnce.Do(func() {
		mdCh = make(chan mdReq, 256) // буфер на bursts
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			for r := range mdCh {
				r.out <- EscapeMarkdown(r.in)
			}
		}()
	})
}

func EscapeMarkdownActor(in string) string {
	ensureMDWorker()
	reply := make(chan string, 1)
	mdCh <- mdReq{in: in, out: reply}
	return <-reply
}

func EscapeMarkdownActorCtx(ctx context.Context, in string) (string, error) {
	ensureMDWorker()
	reply := make(chan string, 1)

	select {
	case mdCh <- mdReq{in: in, out: reply}:
		// ok
	case <-ctx.Done():
		return "", ctx.Err()
	}

	select {
	case res := <-reply:
		return res, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func EscapeMarkdownActorWithTimeout(in string, d time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return EscapeMarkdownActorCtx(ctx, in)
}

var ErrActorClosed = errors.New("markdown actor closed")

func CloseMarkdownActor() error {
	if mdCh == nil {
		return nil
	}
	close(mdCh)
	return nil
}

func EscapeMarkdown(input string) string {
	if len(input) == 0 {
		return input
	}

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