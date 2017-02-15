Release 1.0

1. docs: new README

Backlog

1. monitoring: client connection state machine
1. monitoring: ping https://godoc.org/github.com/hashicorp/yamux#Session.Ping
1. monitoring: prometheus.io integration
1. proxy: WebSockets
1. proxy: UDP
1. proxy: file system
1. proxy: host_header modifier
1. security: certificate signature checks
1. cli: integrated certificate generation

Notes for README

1. HTTP/2
1. Server http.RoundTriper
1. Extensible Proxy architecture
1. Configurable HTTP proxy httputil.ReverseProxy
1. Custom listener and dialer
1. Connection back off
1. Dynamic tunnel management
1. Structured logs, go kit logger compatible

Log levels:

* 0 - Critical, error something went really wrong
* 1 - Info, something important happened
* 2 - Debug
* 3 - Trace, reserved for data transfer logs
