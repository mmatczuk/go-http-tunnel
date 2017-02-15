package log

// Logger is the fundamental interface for all log operations. Log creates a
// log event from keyvals, a variadic sequence of alternating keys and values.
// Implementations must be safe for concurrent use by multiple goroutines. In
// particular, any implementation of Logger that appends to keyvals or
// modifies any of its elements must make a copy first.
type Logger interface {
	Log(keyvals ...interface{}) error
}

// Context is simplified version of go-kit log Context
// https://godoc.org/github.com/go-kit/kit/log#Context.
type Context struct {
	prefix []interface{}
	suffix []interface{}
	logger Logger
}

// NewContext returns a logger that adds prefix before keyvals.
func NewContext(logger Logger) *Context {
	return &Context{
		prefix: make([]interface{}, 0),
		suffix: make([]interface{}, 0),
		logger: logger,
	}
}

// With returns a new Context with keyvals appended to those of the receiver.
func (c *Context) With(keyvals ...interface{}) *Context {
	return &Context{
		prefix: c.prefix,
		suffix: append(c.suffix, keyvals...),
		logger: c.logger,
	}
}

// WithPrefix returns a new Context with keyvals prepended to those of the
// receiver.
func (c *Context) WithPrefix(keyvals ...interface{}) *Context {
	return &Context{
		prefix: append(c.prefix, keyvals...),
		suffix: c.suffix,
		logger: c.logger,
	}
}

// Log adds prefix and suffix to keyvals and calls internal logger.
func (c *Context) Log(keyvals ...interface{}) error {
	var s []interface{}
	s = append(c.prefix, keyvals...)
	s = append(s, c.suffix...)
	return c.logger.Log(s...)
}
