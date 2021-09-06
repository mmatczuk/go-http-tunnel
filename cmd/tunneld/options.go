// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
)

const usage1 string = `Usage: tunneld [OPTIONS]
options:
`

const usage2 string = `
Example:
	tunneld
	tunneld -clients YMBKT3V-ESUTZ2Z-7MRILIJ-T35FHGO-D2DHO7D-FXMGSSR-V4LBSZX-BNDONQ4
	tunneld -httpAddr :8080 -httpsAddr ""
	tunneld -httpsAddr "" -sniAddr ":443" -rootCA client_root.crt -tlsCrt server.crt -tlsKey server.key

Author:
	Written by M. Matczuk (mmatczuk@gmail.com)

Bugs:
	Submit bugs to https://github.com/mmatczuk/go-http-tunnel/issues
`

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage1)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, usage2)
	}
}

// options specify arguments read command line arguments.
type options struct {
	httpAddr     string
	httpsAddr    string
	tunnelAddr   string
	sniAddr      string
	tlsCrt       string
	tlsKey       string
	rootCA       string
	clients      string
	logLevel     int
	version      bool
	natsAddr     string
	natsPort     int
	publishTopic string
}

func parseArgs() *options {
	httpAddr := flag.String("httpAddr", ":80", "Public address for HTTP connections, empty string to disable")
	httpsAddr := flag.String("httpsAddr", ":443", "Public address listening for HTTPS connections, emptry string to disable")
	tunnelAddr := flag.String("tunnelAddr", ":5223", "Public address listening for tunnel client")
	sniAddr := flag.String("sniAddr", "", "Public address listening for TLS SNI connections, empty string to disable")
	tlsCrt := flag.String("tlsCrt", "server.crt", "Path to a TLS certificate file")
	tlsKey := flag.String("tlsKey", "server.key", "Path to a TLS key file")
	rootCA := flag.String("rootCA", "", "Path to the trusted certificate chian used for client certificate authentication, if empty any client certificate is accepted")
	clients := flag.String("clients", "", "Comma-separated list of tunnel client ids, if empty accept all clients")
	logLevel := flag.Int("log-level", 1, "Level of messages to log, 0-3")
	version := flag.Bool("version", false, "Prints tunneld version")
	natsAddr := flag.String("natsAddr", "localhost", "Prints tunneld version")
	natsPort := flag.Int("natsPort", 4222, "Prints tunneld version")
	publishTopic := flag.String("publishTopic", "test", "Prints tunneld version")
	flag.Parse()

	return &options{
		httpAddr:     *httpAddr,
		httpsAddr:    *httpsAddr,
		tunnelAddr:   *tunnelAddr,
		sniAddr:      *sniAddr,
		tlsCrt:       *tlsCrt,
		tlsKey:       *tlsKey,
		rootCA:       *rootCA,
		clients:      *clients,
		logLevel:     *logLevel,
		version:      *version,
		natsAddr:     *natsAddr,
		natsPort:     *natsPort,
		publishTopic: *publishTopic,
	}
}
