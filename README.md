# Tunnel [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/mmatczuk/tunnel) [![Go Report Card](https://goreportcard.com/badge/github.com/mmatczuk/tunnel)](https://goreportcard.com/report/github.com/mmatczuk/tunnel) [![Build Status](http://img.shields.io/travis/mmatczuk/tunnel.svg?style=flat-square)](https://travis-ci.org/mmatczuk/tunnel)

Tunnel is fast and secure server/client package that enables proxying public connections to your local machine over a tunnel connection from the local machine to the public server. In other words you can share your localhost even if it doesn't have a public IP or if it's not reachable from outside.

It uses HTTP/2 protocol for data transport and connection multiplexing.

With tunnel you can proxy:

* HTTP
* TCP
* UNIX sockets
