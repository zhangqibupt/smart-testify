package logger

import (
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

var log *logrus.Logger

func init() {
	log = logrus.New()

	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}

	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
		log.SetReportCaller(true)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	if os.Getenv("LOG_FORMAT") == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{DisableColors: false})
	}

	log.SetOutput(os.Stdout)
}

func GetLogger() *logrus.Logger {
	return log
}
