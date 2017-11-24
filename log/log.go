// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

import (
	"io"
	"log"
	"os"
)

// Logger is the fundamental interface for all log operations. Log creates a
// log event from keyvals, a variadic sequence of alternating keys and values.
// Implementations must be safe for concurrent use by multiple goroutines. In
// particular, any implementation of Logger that appends to keyvals or
// modifies any of its elements must make a copy first.
type Logger interface {
	Log(keyvals ...interface{}) error
}

// NewLogger returns logfmt based logger, printing messages up to log level
// logLevel.
func NewLogger(to string, level int) (Logger, error) {
	var w io.Writer

	switch to {
	case "none":
		return NewNopLogger(), nil
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

	log.SetOutput(w)

	l := NewStdLogger()
	l = NewFilterLogger(l, level)
	return l, nil
}
