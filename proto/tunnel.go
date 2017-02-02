package proto

// Tunnel specifies tunnel entry point. Tunnel map is sent from client to server
// during handshake. Server tries to proxy connections to Host and Addr to
// client.
type Tunnel struct {
	Protocol string
	Host     string
	Auth     string
	Addr     string
}
