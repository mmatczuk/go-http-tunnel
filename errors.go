// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"errors"
	"io/ioutil"
)

var (
	errClientNotSubscribed    = newError("clientNotSubscribed.html", "client not subscribed")
	errClientNotConnected     = newError("clientNotConnected.html", "client not connected")
	errClientAlreadyConnected = newError("clientAlreadyConnected.html", "client already connected")

	errUnauthorised = newError("unauthorised.html", "unauthorised")
)

func newError(fileName string, defaultMsg string) error {
	content, err := ioutil.ReadFile("html/errors/" + fileName)
	if err != nil {
		// handle the case where the file doesn't exist
		return errors.New(defaultMsg)
	}
	return errors.New(string(content))
}
