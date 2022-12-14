# Go HTTP tunnel [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](http://godoc.org/github.com/hons82/go-http-tunnel) [![Go Report Card](https://goreportcard.com/badge/github.com/hons82/go-http-tunnel)](https://goreportcard.com/report/github.com/hons82/go-http-tunnel) [![Build Status](http://img.shields.io/travis/hons82/go-http-tunnel.svg?branch=master)](https://travis-ci.org/hons82/go-http-tunnel) [![Github All Releases](https://img.shields.io/github/downloads/hons82/go-http-tunnel/total.svg)](https://github.com/hons82/go-http-tunnel/releases)

Go HTTP tunnel is a reverse tunnel based on HTTP/2. It enables you to share your localhost when you don't have a public IP.

Features:

* HTTP proxy with [basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication)
* TCP proxy
* [SNI](https://en.wikipedia.org/wiki/Server_Name_Indication) vhost proxy
* Client auto reconnect
* Client management and eviction
* Easy to use CLI

Common use cases:

* Hosting a game server from home
* Developing webhook integrations
* Managing IoT devices

## Project Status

IF YOU WOULD LIKE TO SEE THIS PROJECT MODERNIZED PLEASE [UPVOTE THE ISSUE](https://github.com/mmatczuk/go-http-tunnel/issues/142).

## Installation

Build the latest version.

```bash
$ go get -u github.com/hons82/go-http-tunnel/cmd/...
```

Alternatively [download the latest release](https://github.com/hons82/go-http-tunnel/releases/latest).

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

### Run client:

* Install `tunnel` binary
* Make `.tunnel` directory in your project directory
* Copy `client.key`, `client.crt` to `.tunnel`
* Create configuration file `tunnel.yml` in `.tunnel`
* Start all tunnels

```bash
$ tunnel -config ./tunnel/tunnel.yml start-all
```

### Run server:

* Install `tunneld` binary
* Make `.tunneld` directory
* Copy `server.key`, `server.crt` to `.tunneld`
* Start tunnel server

```bash
$ tunneld -tlsCrt .tunneld/server.crt -tlsKey .tunneld/server.key
```

This will run HTTP server on port `80` and HTTPS (HTTP/2) server on port `443`. If you want to use HTTPS it's recommended to get a properly signed certificate to avoid security warnings.

### Run Server as a Service on Ubuntu using Systemd:

* After completing the steps above successfully, create a new file for your service (you can name it whatever you want, just replace the name below with your chosen name).

``` bash
$ vim tunneld.service
```

* Add the following configuration to the file

```
[Unit]
Description=Go-Http-Tunnel Service
After=network.target
After=network-online.target

[Service]
ExecStart=/path/to/your/tunneld -tlsCrt /path/to/your/folder/.tunneld/server.crt -tlsKey /path/to/your/folder/.tunneld/server.key
TimeoutSec=30
Restart=on-failure
RestartSec=30

[Install]
WantedBy=multi-user.target
```

* Save and exit this file.
* Move this new file to /etc/systemd/system/

```bash
$ sudo mv tunneld.service /etc/systemd/system/
```

* Change the file permission to allow it to run.

```bash
$ sudo chmod u+x /etc/systemd/system/tunneld.service
```

* Start the new service and make sure you don't get any errors, and that your client is able to connect.

```bash
$ sudo systemctl start tunneld.service
```

* You can stop the service with:

```bash
$ sudo systemctl stop tunneld.service
```

* Finally, if you want the service to start automatically when the server is rebooted, you need to enable it.

```bash
$ sudo systemctl enable tunneld.service
```

There are many more options for systemd services, and this is by not means an exhaustive configuration file.

## Configuration - Client

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
      tls:
  	    proto: sni
  	    addr: localhost:443
  	    host: tls.my-tunnel-host.com
```

Configuration options:

* `server_addr`: server TCP address, i.e. `54.12.12.45:5223`
* `tls_crt`: path to client TLS certificate, *default:* `client.crt` *in the config file directory*
* `tls_key`: path to client TLS certificate key, *default:* `client.key` *in the config file directory*
* `root_ca`: path to trusted root certificate authority pool file, if empty any server certificate is accepted
*  `tunnels / [name]`
    * `proto`: tunnel protocol, `http`, `tcp` or `sni`
    * `addr`: forward traffic to this local port number or network address, for `proto=http` this can be full URL i.e. `https://machine/sub/path/?plus=params`, supports URL schemes `http` and `https`
    * `auth`: (`proto=http`) (optional) basic authentication credentials to enforce on tunneled requests, format `user:password`
    * `host`: (`proto=http`, `proto=sni`) hostname to request (requires reserved name and DNS CNAME)
    * `remote_addr`: (`proto=tcp`) bind the remote TCP address
* `backoff`
    * `interval`: how long client would wait before redialing the server if connection was lost, exponential backoff initial interval, *default:* `500ms`
    * `multiplier`: interval multiplier if reconnect failed, *default:* `1.5`
    * `max_interval`: maximal time client would wait before redialing the server, *default:* `1m`
    * `max_time`: maximal time client would try to reconnect to the server if connection was lost, set `0` to never stop trying, *default:* `15m`
* `keep_alive`
     * `interval`: the amount of time to wait between sending keepalive packets, *default:* `25s`

## Configuration - Server

* `httpAddr`: Public address for HTTP connections, empty string to disable,  *default:* `:80`
* `httpsAddr`: Public address listening for HTTPS connections, emptry string to disable, *default:* `:443`
* `tunnelAddr`: Public address listening for tunnel client, *default:* `:5223`
* `apiAddr`: Public address for HTTP API to get info about the tunnels, *default:* `:5091`
* `sniAddr`: Public address listening for TLS SNI connections, empty string to disable
* `tlsCrt`: Path to a TLS certificate file, *default:* `server.crt`
* `tlsKey`: Path to a TLS key file, *default:* `server.key`
* `rootCA`: Path to the trusted certificate chian used for client certificate authentication, if empty any client certificate is accepted
* `clients`: Path to a properties file that contains a list of 'host=tunnelClientId's, if empty accept all clients
* `keepAlive`: the amount of time to wait between sending keepalive packets *default:* `45s`
* `logLevel`: Level of messages to log, 0-3, *default:* 1

If both `httpAddr` and `httpsAddr` are configured, an automatic redirect to the secure channel will be established using an `http.StatusMovedPermanently` (301)

### Custom error pages

Just copy the `html` folder from this repository into the folder of the tunnel-server to have a starting point. In the `html/errors` folder you'll find a sample page for each error that is currently customisable which you'll be able to change according to your needs.

## Server API

If the `apiAddr` is properly set, the tunnel server offers a simple API to query its state.

### /api/clients/list

Returns a list of `clients` together with a list of open tunnels in JSON format.

```json
[
    {
        "Id": "BHXWUUT-A6IYDWI-2BSIC5A-...",
        "Listeners": [
            {
                "Network": "tcp",
                "Addr": "192.0.2.1:25"
            }
        ],
        "Hosts": [
            "tunnel1.my-tunnel-host.com"
        ]
    }
]
```

## How it works

A client opens TLS connection to a server. The server accepts connections from known clients only. The client is recognized by its TLS certificate ID. The server is publicly available and proxies incoming connections to the client. Then the connection is further proxied in the client's network.

The tunnel is based HTTP/2 for speed and security. There is a single TCP connection between client and server and all the proxied connections are multiplexed using HTTP/2.

## Donation

If this project help you reduce time to develop, you can give me a cup of coffee.

[![paypal](https://www.paypalobjects.com/en_US/IT/i/btn/btn_donateCC_LG.gif)](https://www.paypal.com/donate?hosted_button_id=E74HP49TAAUQ2)

A GitHub star is always appreciated!

## License

Copyright (C) 2017 Michał Matczuk

This project is distributed under the AGPL-3 license. See the [LICENSE](https://github.com/hons82/go-http-tunnel/blob/master/LICENSE) file for details. If you need an enterprice license contact me directly.
