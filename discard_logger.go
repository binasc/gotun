package main

import (
	"io"
)

type DiscardLogger struct {}

func NewDiscardLogger(out io.Writer, prefix string, flag int) *DiscardLogger {
	return &DiscardLogger{}
}

func (l *DiscardLogger) Printf(format string, v ...interface{}) {
}
