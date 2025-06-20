package persistence

import "ximanager/sources/tracing"

type gormtracer struct {
	logger *tracing.Logger
}

func (w *gormtracer) Printf(format string, args ...interface{}) {
	w.logger.D(format, args...)
}