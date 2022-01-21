package kubescaler

import (
	"io/ioutil"
	"log"
)

type Logger interface {
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
}

type defaultLogger struct {
	infoLogger  *log.Logger
	debugLogger *log.Logger
	errorLogger *log.Logger
}

func newDefaultLogger(infoLogger *log.Logger, debugLogger *log.Logger, errorLogger *log.Logger) *defaultLogger {
	logger := &defaultLogger{
		infoLogger:  infoLogger,
		debugLogger: debugLogger,
		errorLogger: errorLogger,
	}
	if infoLogger == nil {
		logger.infoLogger = log.New(ioutil.Discard, "", 0)
	}
	if debugLogger == nil {
		logger.debugLogger = log.New(ioutil.Discard, "", 0)
	}
	if errorLogger == nil {
		logger.errorLogger = log.New(ioutil.Discard, "", 0)
	}
	return logger
}

func (l *defaultLogger) Info(v ...interface{}) {
	l.infoLogger.Print(v...)
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	l.infoLogger.Printf(format, v...)
}

func (l *defaultLogger) Debug(v ...interface{}) {
	l.debugLogger.Print(v...)
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	l.debugLogger.Printf(format, v...)
}

func (l *defaultLogger) Error(v ...interface{}) {
	l.errorLogger.Print(v...)
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	l.errorLogger.Printf(format, v...)
}
