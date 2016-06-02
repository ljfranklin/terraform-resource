package logger

import (
	"fmt"
	"io"
)

type Logger struct {
	Sink           io.Writer
	sectionColor   color
	sectionMessage string
}

type color int

var err color = 31     // red
var success color = 32 // green
var warn color = 33    // yellow
var info color = 34    // blue

func (l Logger) Info(message string) {
	l.logWithColor(message, info)
}

func (l Logger) Success(message string) {
	l.logWithColor(message, success)
}

func (l Logger) Warn(message string) {
	l.logWithColor(message, warn)
}

func (l Logger) Error(message string) {
	l.logWithColor(message, err)
}

func (l *Logger) InfoSection(message string) {
	l.sectionMessage = message
	l.sectionColor = info
	l.startSection()
}

func (l *Logger) SuccessSection(message string) {
	l.sectionMessage = message
	l.sectionColor = success
	l.startSection()
}

func (l *Logger) WarnSection(message string) {
	l.sectionMessage = message
	l.sectionColor = warn
	l.startSection()
}

func (l *Logger) ErrorSection(message string) {
	l.sectionMessage = message
	l.sectionColor = err
	l.startSection()
}

func (l Logger) logWithColor(message string, c color) {
	coloredMessage := fmt.Sprintf("\033[%dm%s\033[0m\n", c, message)
	l.Sink.Write([]byte(coloredMessage))
}

func (l *Logger) startSection() {
	l.logWithColor(fmt.Sprintf("▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ %s ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼", l.sectionMessage), l.sectionColor)
}

func (l *Logger) EndSection() {
	l.logWithColor(fmt.Sprintf("▲ ▲ ▲ ▲ ▲ ▲ ▲ ▲ ▲ ▲ %s ▲ ▲ ▲ ▲ ▲ ▲ ▲ ▲ ▲ ▲", l.sectionMessage), l.sectionColor)
	l.sectionColor = 0
	l.sectionMessage = ""
}
