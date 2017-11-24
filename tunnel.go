// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import "time"

var (
	// DefaultTimeout specifies a general purpose timeout.
	DefaultTimeout = 10 * time.Second
	// DefaultPingTimeout specifies a ping timeout.
	DefaultPingTimeout = 500 * time.Millisecond
)
