package tracing

import (
	"time"
)

func ReportExecutionForRE[R any, E error](log *Logger, action func() (R, E), report func(l *Logger)) (R, E) {
	start := time.Now()
	result, err := action()
	report(log.With(ExecutionTime, time.Since(start).String()))
	return result, err
}

func ReportExecutionForR[R any](log *Logger, action func() R, report func(l *Logger)) R {
	start, result := time.Now(), action()
	report(log.With(ExecutionTime, time.Since(start).String()))
	return result
}

func ReportExecutionForRIn[R any](log *Logger, action func() R, report func(l *Logger, result R)) R {
	start := time.Now()
	result := action()
	report(log.With(ExecutionTime, time.Since(start).String()), result)
	return result
}

func ReportExecution(log *Logger, action func(), report func(l *Logger)) {
	start := time.Now()
	action()
	report(log.With(ExecutionTime, time.Since(start).String()))
}