package project

import (
	"github.com/pterm/pterm"
)

var JBLog = NewLogger()

type Logger interface {
	Fatalf(format string, args ...interface{})
}

func NewLogger() Logger {
	return &StdLogger{}
}

type StdLogger struct {
}

func (l *StdLogger) Fatalf(format string, args ...interface{}) {
	pterm.Fatal.Printf(format, args...)
}

func (l *StdLogger) Printf(format string, args ...interface{}) {
	pterm.DefaultBasicText.Printf(format, args...)
}
