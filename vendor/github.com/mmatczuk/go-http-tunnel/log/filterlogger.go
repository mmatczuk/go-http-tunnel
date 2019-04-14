// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

type filterLogger struct {
	level  int
	logger Logger
}

// NewFilterLogger returns a Logger that accepts only log messages with
// "level" value <= level. Currently there are four levels 0 - error, 1 - info,
// 2 - debug, 3 - trace.
func NewFilterLogger(logger Logger, level int) Logger {
	return filterLogger{
		level:  level,
		logger: logger,
	}
}

func (p filterLogger) Log(keyvals ...interface{}) error {
	for i := 0; i < len(keyvals); i += 2 {
		k := keyvals[i]
		s, ok := k.(string)
		if !ok {
			continue
		}
		if s != "level" {
			continue
		}

		if i+1 >= len(keyvals) {
			break
		}
		v := keyvals[i+1]
		level, ok := v.(int)
		if !ok {
			break
		}

		if level > p.level {
			return nil
		}
	}

	return p.logger.Log(keyvals...)
}
