# Tunnel [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/mmatczuk/go-http-tunnel) [![Go Report Card](https://goreportcard.com/badge/github.com/mmatczuk/go-http-tunnel)](https://goreportcard.com/report/github.com/mmatczuk/go-http-tunnel) [![Build Status](http://img.shields.io/travis/mmatczuk/go-http-tunnel.svg?style=flat-square)](https://travis-ci.org/mmatczuk/go-http-tunnel.svg?branch=master) [![Github All Releases](https://img.shields.io/github/downloads/mmatczuk/go-http-tunnel/total.svg)](https://github.com/mmatczuk/go-http-tunnel/releases)

Tunnel is fast and secure client/server package that enables proxying public connections to your local machine over a tunnel connection from the local machine to the public server. **It enables you to share your localhost when you don't have a public IP or you are hidden by a firewall**.

It can help you:

* Demo without deploying
* Simplify mobile device testing
* Build webhook integrations with ease
* Run personal cloud services from your own private network

It is based on HTTP/2 for speed and security. Server accepts TLS connection from known clients, client is recognised by it's TLS certificate id. Server can protect HTTP tunnels with [basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication).

## Installation

Download latest release from [here](https://github.com/mmatczuk/go-http-tunnel/releases/latest).  The release contains two executables:

* `tunneld` - the tunnel server, to be run on publicly available host like AWS or GCE
* `tunnel` - the tunnel client, to be run on your local machine or in your private network

To get help on the command parameters run `tunneld -h` or `tunnel -h`.

## Configuration

The tunnel client `tunnel` requires configuration file, by default it will try reading `tunnel.yml` in your current working directory. If you want to specify other file use `-config` flag.

Sample configuration that exposes:

* `localhost:8080` as `webui.my-tunnel-host.com` 
* host in private network for ssh connections

looks like this

```yaml
    server_addr: SERVER_IP:4443
    insecure_skip_verify: true
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

* `server_addr`: server TCP address, i.e. `54.12.12.45:4443`
* `insecure_skip_verify`: controls whether a client verifies the server's certificate chain and host name, if using self signed certificates must be set to `true`, *default:* `false`
* `tls_crt`: path to client TLS certificate, *default:* `client.crt` *in the config file directory*
* `tls_key`: path to client TLS certificate key, *default:* `client.key` *in the config file directory*
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

## Running

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
* Get client identifier (`tunnel -config ./tunnel/tunnel.yml id`), identifier should look like this `YMBKT3V-ESUTZ2Z-7MRILIJ-T35FHGO-D2DHO7D-FXMGSSR-V4LBSZX-BNDONQ4`
* Start tunnel server 

```bash
$ tunneld -tlsCrt .tunneld/server.crt -tlsKey .tunneld/server.key -clients YMBKT3V-ESUTZ2Z-7MRILIJ-T35FHGO-D2DHO7D-FXMGSSR-V4LBSZX-BNDONQ4
``` 

This will run HTTP server on port `80` and HTTPS (HTTP/2) server on port `443`. If you want to use HTTPS it's recommended to get a properly signed certificate to avoid security warnings. 

## Using as a library

Install the package:

```bash
$ go get -u github.com/mmatczuk/go-http-tunnel
```

The `tunnel` package is designed to be simple, extensible, with little dependencies. It is based on HTTP/2 for client server connectivity, this avoids usage of third party tools for multiplexing tunneled connections. HTTP/2 is faster, more stable and much more tested then any other multiplexing technology. You may see [benchmark](benchmark) comparing the `tunnel` package to a koding tunnel.

The `tunnel` package:

* custom dialer and listener for `Client` and `Server`
* easy modifications of HTTP proxy (based on [ReverseProxy](https://golang.org/pkg/net/http/httputil/#ReverseProxy))
* proxy anything, [ProxyFunc](https://godoc.org/github.com/mmatczuk/go-http-tunnel#ProxyFunc) architecture
* structured logs with go-kit compatible minimal logger interface

See:

* [ClientConfig](https://godoc.org/github.com/mmatczuk/go-http-tunnel#ClientConfig)
* [ServerConfig](https://godoc.org/github.com/mmatczuk/go-http-tunnel#ServerConfig)
* [ControlMessage](https://godoc.org/github.com/mmatczuk/go-http-tunnel/proto#ControlMessage)

## License

Copyright (C) 2017 Michał Matczuk

This project is distributed under the BSD-3 license. See the [LICENSE](https://github.com/mmatczuk/go-http-tunnel/blob/master/LICENSE) file for details.

GitHub star is always appreciated!
