package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Setup initializes the global zerolog logger based on environment configuration.
//   - level: log level string (trace, debug, info, warn, error, fatal, panic)
//   - format: "json" for production, "pretty" for human-readable dev output
//
// Returns the configured logger instance.
func Setup(level, format string) zerolog.Logger {
	var writer io.Writer

	if format == "pretty" {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	} else {
		writer = os.Stdout
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(lvl)

	log := zerolog.New(writer).
		With().
		Timestamp().
		Caller().
		Logger()

	return log
}
