package cmd

import (
	"io"
	"os"
	"time"

	kitlog "github.com/go-kit/kit/log"
	"github.com/mmatczuk/tunnel/log"
)

// NewLogger returns logfmt based logger, printing messages up to log level
// logLevel.
func NewLogger(to string, level int) (log.Logger, error) {
	var w io.Writer

	switch to {
	case "none":
		return log.NewNopLogger(), nil
	case "stdout":
		w = os.Stdout
	case "stderr":
		w = os.Stderr
	default:
		f, err := os.Create(to)
		if err != nil {
			return nil, err
		}
		w = f
	}

	var logger kitlog.Logger
	logger = kitlog.NewJSONLogger(kitlog.NewSyncWriter(w))
	logger = kitlog.NewContext(logger).WithPrefix("time", kitlog.Timestamp(time.Now))
	logger = log.NewFilterLogger(logger, level)
	return logger, nil
}
