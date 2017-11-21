// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tunnel

import "time"

var (
	// DefaultTimeout specifies a general purpose timeout.
	DefaultTimeout = 10 * time.Second
	// DefaultPingTimeout specifies a ping timeout.
	DefaultPingTimeout = 500 * time.Millisecond

	// DefaultKeepAliveIdleTime specifies how long connection can be idle
	// before sending keepalive message.
	DefaultKeepAliveIdleTime = 15 * time.Minute
	// DefaultKeepAliveCount specifies maximal number of keepalive messages
	// sent before marking connection as dead.
	DefaultKeepAliveCount = 8
	// DefaultKeepAliveInterval specifies how often retry sending keepalive
	// messages when no response is received.
	DefaultKeepAliveInterval = 5 * time.Second
)
