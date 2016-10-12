# h2tun [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/mmatczuk/h2tun) [![Go Report Card](https://goreportcard.com/badge/github.com/mmatczuk/h2tun)](https://goreportcard.com/report/github.com/mmatczuk/h2tun) [![Build Status](http://img.shields.io/travis/mmatczuk/h2tun.svg?style=flat-square)](https://travis-ci.org/mmatczuk/h2tun)

h2tun is fast and secure server/client package that enables proxying public connections to your local machine over a tunnel connection from the local machine to the public server. In other words you can share your localhost even if it doesn't have a public IP or if it's not reachable from outside.

It uses HTTP/2 protocol for data transport and connection multiplexing.

With h2tun you can proxy:

* HTTP
* TCP
* UNIX sockets

## Benchmark

h2tun is benchmarked against [koding tunnel](https://github.com/koding/tunnel). h2tun proves to be more stable, it can handle greater throughput with better latencies. [See benchmark report](benchmark/report/README.md) for more details.
