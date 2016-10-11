# h2tun

h2tun is fast and secure server/client package that enables proxying public connections to your local machine over a tunnel connection from the local machine to the public server. In other words you can share your localhost even if it doesn't have a public IP or if it's not reachable from outside.

It uses HTTP/2 protocol for data transport and connection multiplexing.

With h2tun you can proxy:

* HTTP
* TCP
* UNIX sockets

## Benchmark

h2tun is benchmarked against [koding tunnel](https://github.com/koding/tunnel). h2tun proves to be more stable, it can handle greater throughput with better latencies. [See benchmark report](benchmark/report/README.md) for more details.
