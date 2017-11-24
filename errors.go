// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import "errors"

var (
	errClientNotSubscribed    = errors.New("client not subscribed")
	errClientNotConnected     = errors.New("client not connected")
	errClientAlreadyConnected = errors.New("client already connected")

	errUnauthorised = errors.New("unauthorised")
)
