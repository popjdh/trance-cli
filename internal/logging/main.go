package logging

import (
	"fmt"
	"io"
	"strings"
)

type LogMode int

const (
	LogModeInPlace LogMode = iota
	LogModeAppend
)

type LoggerState int

const (
	LoggerStateNewLine LoggerState = iota
	LoggerStateOutOldLine
	LoggerStateErrOldLine
)

type Logger struct {
	OutWriter io.Writer
	ErrWriter io.Writer
	State     LoggerState
}

func (logger *Logger) SetState(state LoggerState) {
	logger.State = state
}

func (logger *Logger) PrintfOut(mode LogMode, endLF bool, format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	switch mode {
	case LogModeInPlace:
		if logger.State == LoggerStateErrOldLine {
			_, _ = fmt.Fprintf(logger.ErrWriter, "\n")
		}
		message = "\r" + message + "\x1b[K"
	case LogModeAppend:
		if logger.State == LoggerStateOutOldLine {
			_, _ = fmt.Fprintf(logger.OutWriter, "\n")
		} else if logger.State == LoggerStateErrOldLine {
			_, _ = fmt.Fprintf(logger.ErrWriter, "\n")
		}
	}
	_, _ = fmt.Fprintf(logger.OutWriter, message)
	if endLF {
		_, _ = fmt.Fprintf(logger.OutWriter, "\n")
		logger.State = LoggerStateNewLine
	} else {
		lastIndex := strings.LastIndex(message, "\n")
		if lastIndex == -1 || lastIndex < len(message)-1 {
			logger.State = LoggerStateOutOldLine
		} else {
			logger.State = LoggerStateNewLine
		}
	}
}

func (logger *Logger) PrintfErr(mode LogMode, endLF bool, format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	switch mode {
	case LogModeInPlace:
		if logger.State == LoggerStateOutOldLine {
			_, _ = fmt.Fprintf(logger.OutWriter, "\n")
		}
		message = "\r" + message + "\x1b[K"
	case LogModeAppend:
		if logger.State == LoggerStateOutOldLine {
			_, _ = fmt.Fprintf(logger.OutWriter, "\n")
		} else if logger.State == LoggerStateErrOldLine {
			_, _ = fmt.Fprintf(logger.ErrWriter, "\n")
		}
	}
	_, _ = fmt.Fprintf(logger.ErrWriter, message)
	if endLF {
		_, _ = fmt.Fprintf(logger.ErrWriter, "\n")
		logger.State = LoggerStateNewLine
	} else {
		lastIndex := strings.LastIndex(message, "\n")
		if lastIndex == -1 || lastIndex < len(message)-1 {
			logger.State = LoggerStateErrOldLine
		} else {
			logger.State = LoggerStateNewLine
		}
	}
}
