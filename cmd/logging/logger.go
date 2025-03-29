// Package logging provides centralized logging functionality for the application.
package logging

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"minecharts/cmd/config"
)

// Logger is the global logger instance
var Logger *logrus.Logger

// Field represents a log field with key and value
type Field struct {
	Key   string
	Value interface{}
}

// Init initializes the logger with the configured log level
func Init() {
	InitStructuredLogging()
	// Create new logger
	Logger = logrus.New()

	// Set output to stdout
	Logger.SetOutput(os.Stdout)

	// Set log format
	switch strings.ToLower(config.LogFormat) {
	case "text":
		Logger.SetFormatter(&logrus.TextFormatter{
			DisableColors:    false,
			DisableTimestamp: false,
			FullTimestamp:    true,
			TimestampFormat:  "2006/01/02 15:04:05",
		})
	case "json":
		Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006/01/02 15:04:05",
		})
	default:
		Logger.SetFormatter(&logrus.TextFormatter{
			DisableColors:    false,
			DisableTimestamp: false,
			FullTimestamp:    true,
			TimestampFormat:  "2006/01/02 15:04:05",
		})
		Logger.Warnf("Invalid log format %s, using text format", config.LogFormat)
	}

	// Set log level from configuration
	level, err := logrus.ParseLevel(strings.ToLower(config.LogLevel))
	if err != nil {
		// Default to info level if parsing fails
		level = logrus.InfoLevel
		Logger.Warnf("Invalid log level %s, using info level", config.LogLevel)
	}
	Logger.SetLevel(level)

	Logger.Infof("Logger initialized with level: %s", level.String())
}

// WithFields returns a new entry with the specified fields
func WithFields(fields ...Field) *logrus.Entry {
	logrusFields := logrus.Fields{}
	for _, field := range fields {
		logrusFields[field.Key] = field.Value
	}
	return Logger.WithFields(logrusFields)
}

// Field creation helpers
func F(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Convenience wrapper functions for common logging levels
func Trace(msg string) {
	Logger.Trace(msg)
}

func Debug(msg string) {
	Logger.Debug(msg)
}

func Info(msg string) {
	Logger.Info(msg)
}

func Warn(msg string) {
	Logger.Warn(msg)
}

func Error(msg string) {
	Logger.Error(msg)
}

func Fatal(msg string) {
	Logger.Fatal(msg)
}

func Panic(msg string) {
	Logger.Panic(msg)
}

// Formatted convenience wrapper functions
func Tracef(format string, args ...interface{}) {
	Logger.Tracef(format, args...)
}

func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
}

func Panicf(format string, args ...interface{}) {
	Logger.Panicf(format, args...)
}
