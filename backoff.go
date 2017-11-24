// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import "time"

// Backoff defines behavior of staggering reconnection retries.
type Backoff interface {
	// Next returns the duration to sleep before retrying to reconnect.
	// If the returned value is negative, the retry is aborted.
	NextBackOff() time.Duration

	// Reset is used to signal a reconnection was successful and next
	// call to Next should return desired time duration for 1st reconnection
	// attempt.
	Reset()
}
