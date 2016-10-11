1. Connection exponential backoff on client
1. `ClientState` changes channel, on both client and server
1. WebSockets proxing
1. UDP and IP proxing
1. Default proxy functions
1. Dynamic `AllowedClient` management
1. Client driven configuration, on connect client sends it's configuration, server just needs to know the certificate id
1. Ping and RTT, like https://godoc.org/github.com/hashicorp/yamux#Session.Ping
1. Stream compression
1. `ControlMessage` `String()` function for better logging
1. Use of `sync.Pool` to avoid allocations of `ControlMessage`
1. Client and server commands (hcl configuration?)