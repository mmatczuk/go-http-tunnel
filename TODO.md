1. Dynamic `AllowedClient` management
1. Client driven configuration, on connect client sends it's configuration, server just needs to know the certificate id
1. Cli and file configuration https://ngrok.com/docs#config
1. Basic auth on server
1. README update
1. WebSockets proxing
1. Ping and RTT, like https://godoc.org/github.com/hashicorp/yamux#Session.Ping
1. `ClientState` changes channel, on both client and server
1. URL prefix based routing, like urlprefix tag in fabio https://github.com/eBay/fabio/wiki/Quickstart
1. Use of `sync.Pool` to avoid allocations of `ControlMessage`
1. Stream compression
1. UDP and IP proxing