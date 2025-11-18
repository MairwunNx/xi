package tracing

import "time"

func ProfilePoint(log *Logger, msg, opname string, fields ...any) func() {
	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		log.D(msg, append(fields, "elapsed_ms", elapsed.Milliseconds())...)
	}
}