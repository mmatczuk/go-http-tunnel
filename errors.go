// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import "errors"

var (
	errClientManyConnections  = errors.New("controller has many connections")
	errClientNotSubscribed    = errors.New("controller not subscribed")
	errClientNotConnected     = errors.New("controller not connected")
	errClientAlreadyConnected = errors.New("controller already connected")

	errUnauthorised = errors.New("unauthorised")
)
