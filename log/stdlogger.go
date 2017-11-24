// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

import (
	"log"
)

type stdLogger struct{}

// NewStdLogger returns logger based on standard "log" package.
func NewStdLogger() Logger { return stdLogger{} }

func (p stdLogger) Log(keyvals ...interface{}) error {
	log.Println(keyvals...)
	return nil
}
