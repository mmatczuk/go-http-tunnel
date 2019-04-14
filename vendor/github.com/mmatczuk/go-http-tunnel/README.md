# Go HTTP tunnel [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](http://godoc.org/github.com/mmatczuk/go-http-tunnel) [![Go Report Card](https://goreportcard.com/badge/github.com/mmatczuk/go-http-tunnel)](https://goreportcard.com/report/github.com/mmatczuk/go-http-tunnel) [![Build Status](http://img.shields.io/travis/mmatczuk/go-http-tunnel.svg?branch=master)](https://travis-ci.org/mmatczuk/go-http-tunnel) [![Github All Releases](https://img.shields.io/github/downloads/mmatczuk/go-http-tunnel/total.svg)](https://github.com/mmatczuk/go-http-tunnel/releases)

Go HTTP tunnel is a reverse tunnel based on HTTP/2. It enables you to share your localhost when you don't have a public IP.

Features:

* HTTP proxy with [basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication)
* TCP proxy
* Client auto reconnect
* Client management and eviction
* Easy to use CLI

Common use cases:

* Hosting a game server from home
* Developing webhook integrations
* Managing IoT devices

## Installation

Build the latest version.

```bash
$ go get -u github.com/mmatczuk/go-http-tunnel/cmd/...
```

Alternatively [download the latest release](https://github.com/mmatczuk/go-http-tunnel/releases/latest).

## Running

There are two executables:

* `tunneld` - the tunnel server, to be run on publicly available host like AWS or GCE
* `tunnel` - the tunnel client, to be run on your local machine or in your private network

To get help on the command parameters run `tunneld -h` or `tunnel -h`.

Tunnel requires TLS certificates for both client and server.

```bash
$ openssl req -x509 -nodes -newkey rsa:2048 -sha256 -keyout client.key -out client.crt
$ openssl req -x509 -nodes -newkey rsa:2048 -sha256 -keyout server.key -out server.crt
```

Run client:

* Install `tunnel` binary
* Make `.tunnel` directory in your project directory
* Copy `client.key`, `client.crt` to `.tunnel`
* Create configuration file `tunnel.yml` in `.tunnel`
* Start all tunnels

```bash
$ tunnel -config ./tunnel/tunnel.yml start-all
```

Run server:

* Install `tunneld` binary
* Make `.tunneld` directory
* Copy `server.key`, `server.crt` to `.tunneld`
* Start tunnel server

```bash
$ tunneld -tlsCrt .tunneld/server.crt -tlsKey .tunneld/server.key
```

This will run HTTP server on port `80` and HTTPS (HTTP/2) server on port `443`. If you want to use HTTPS it's recommended to get a properly signed certificate to avoid security warnings.

## Configuration

The tunnel client `tunnel` requires configuration file, by default it will try reading `tunnel.yml` in your current working directory. If you want to specify other file use `-config` flag.

Sample configuration that exposes:

* `localhost:8080` as `webui.my-tunnel-host.com`
* host in private network for ssh connections

looks like this

```yaml
    server_addr: SERVER_IP:5223
    tunnels:
      webui:
        proto: http
        addr: localhost:8080
        auth: user:password
        host: webui.my-tunnel-host.com
      ssh:
        proto: tcp
        addr: 192.168.0.5:22
        remote_addr: 0.0.0.0:22
```

Configuration options:

* `server_addr`: server TCP address, i.e. `54.12.12.45:5223`
* `tls_crt`: path to client TLS certificate, *default:* `client.crt` *in the config file directory*
* `tls_key`: path to client TLS certificate key, *default:* `client.key` *in the config file directory*
* `root_ca`: path to trusted root certificate authority pool file, if empty any server certificate is accepted
*  `tunnels / [name]`
    * `proto`: tunnel protocol, `http` or `tcp`
    * `addr`: forward traffic to this local port number or network address, for `proto=http` this can be full URL i.e. `https://machine/sub/path/?plus=params`, supports URL schemes `http` and `https`
    * `auth`: (`proto=http`) (optional) basic authentication credentials to enforce on tunneled requests, format `user:password`
    * `host`: (`proto=http`) hostname to request (requires reserved name and DNS CNAME)
    * `remote_addr`: (`proto=tcp`) bind the remote TCP address
* `backoff`
    * `interval`: how long client would wait before redialing the server if connection was lost, exponential backoff initial interval, *default:* `500ms`
    * `multiplier`: interval multiplier if reconnect failed, *default:* `1.5`
    * `max_interval`: maximal time client would wait before redialing the server, *default:* `1m`
    * `max_time`: maximal time client would try to reconnect to the server if connection was lost, set `0` to never stop trying, *default:* `15m`

## How it works
Client opens a TLS connection to a server. Server accepts connections from known clients only, client is recognised by it's TLS certificate ID. The server is publicly available and proxies incoming connections to the client. Then the connection is further proxied in the client's network. 

Tunnel is based HTTP/2 for speed and security. There is a single TCP connection between client and server and all the proxied connections are multiplexed using HTTP/2.

## Donation

If this project help you reduce time to develop, you can give me a cup of coffee.

[![paypal](https://www.paypalobjects.com/en_US/i/btn/btn_donateCC_LG.gif)](https://www.paypal.com/cgi-bin/webscr?cmd=_donations&business=RMM46NAEY7YZ6&lc=US&item_name=go%2dhttp%2dtunnel&currency_code=USD&bn=PP%2dDonationsBF%3abtn_donateCC_LG%2egif%3aNonHosted)

A GitHub star is always appreciated!

## License

Copyright (C) 2017 Michał Matczuk

This project is distributed under the AGPL-3 license. See the [LICENSE](https://github.com/mmatczuk/go-http-tunnel/blob/master/LICENSE) file for details. If you need an enterprice license contact me directly.
