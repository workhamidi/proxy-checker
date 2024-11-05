package main

import (
	"github.com/sirupsen/logrus"
)

func initLogger() {
	if verbosityLevel != 0 {
		switch verbosityLevel {
		case 4:
			log.SetLevel(logrus.TraceLevel)
		case 3:
			log.SetLevel(logrus.DebugLevel)
		case 2:
			log.SetLevel(logrus.WarnLevel)
		case 1:
			log.SetLevel(logrus.ErrorLevel)
		default:
			log.SetLevel(logrus.ErrorLevel)
		}

		log.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	}
}


func logInfof(format string, args ...interface{}) {
	if verbosityLevel != 0 {
		log.Infof(format, args...)
	}
}

func logDebugf(format string, args ...interface{}) {
	if verbosityLevel != 0 {
		log.Debugf(format, args...)
	}
}

func logWarnf(format string, args ...interface{}) {
	if verbosityLevel != 0 {
		log.Warnf(format, args...)
	}
}

func logErrorf(format string, args ...interface{}) {
	if verbosityLevel != 0 {
		log.Errorf(format, args...)
	}
}
