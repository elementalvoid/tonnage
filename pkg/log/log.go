package log

import (
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

// Logger is a configured logrus logger.
var Logger = log.New()
var summaryOnly, json, colorAlways, debug bool

func init() {
	flag.BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	flag.BoolVarP(&colorAlways, "color-always", "C", false, "Force color output in all cases")
	flag.BoolVar(&json, "json", false, "Enable JSON formatted logging")
	flag.BoolVarP(&summaryOnly, "summary", "s", false, "Show only the cummulative cluster summary")
}

// Configure configures the logger based on flags.
func Configure() {
	if json {
		Logger.Formatter = &log.JSONFormatter{}
	} else {
		Logger.Formatter = &log.TextFormatter{
			ForceColors:   colorAlways,
			FullTimestamp: true,
		}
	}

	if debug {
		Logger.SetLevel(log.DebugLevel)
	}
}
