package h2tun

// ControlMessage is sent from server to client to establish tunneled connection.
type ControlMessage struct {
	Action    Action
	Protocol  Type
	LocalPort int
}

// Action represents type of ControlMsg request.
type Action int

// ControlMessage actions.
const (
	RequestClientSession Action = iota + 1
)

// Type represents tunneled connection type.
type Type int

// ControlMessage protocols.
const (
	HTTP Type = iota + 1
	WS
	RAW
)
