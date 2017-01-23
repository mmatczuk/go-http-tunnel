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
