package cmd

import (
	"io"
	golog "log"
	"os"

	"github.com/mmatczuk/go-http-tunnel/log"
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

	golog.SetOutput(w)

	var logger log.Logger
	logger = log.NewStdLogger()
	logger = log.NewFilterLogger(logger, level)
	return logger, nil
}
