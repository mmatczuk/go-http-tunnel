package tunnel

import "errors"

var (
	errClientNotSubscribed    = errors.New("client not subscribed")
	errClientNotConnected     = errors.New("client not connected")
	errClientAlreadyConnected = errors.New("client already connected")
)
