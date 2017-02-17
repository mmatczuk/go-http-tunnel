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
	tuneld -clients YMBKT3V-ESUTZ2Z-7MRILIJ-T35FHGO-D2DHO7D-FXMGSSR-V4LBSZX-BNDONQ4
	tuneld -httpAddr :8080 -httpsAddr "" -clients YMBKT3V-ESUTZ2Z-7MRILIJ-T35FHGO-D2DHO7D-FXMGSSR-V4LBSZX-BNDONQ4

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
	debug      bool
	httpAddr   string
	httpsAddr  string
	tunnelAddr string
	tlsCrt     string
	tlsKey     string
	clients    string
	logTo      string
	logLevel   int
	version    bool
}

func parseArgs() *options {
	debug := flag.Bool("debug", false, "Starts gops agent")
	httpAddr := flag.String("httpAddr", ":80", "Public address for HTTP connections, empty string to disable")
	httpsAddr := flag.String("httpsAddr", ":443", "Public address listening for HTTPS connections, emptry string to disable")
	tunnelAddr := flag.String("tunnelAddr", ":4443", "Public address listening for tunnel client")
	tlsCrt := flag.String("tlsCrt", "server.crt", "Path to a TLS certificate file")
	tlsKey := flag.String("tlsKey", "server.key", "Path to a TLS key file")
	clients := flag.String("clients", "", "Comma-separated list of tunnel client ids")
	logTo := flag.String("log", "stdout", "Write log messages to this file, file name or 'stdout', 'stderr', 'none'")
	logLevel := flag.Int("log-level", 1, "Level of messages to log, 0-3")
	version := flag.Bool("version", false, "Prints tunneld version")
	flag.Parse()

	return &options{
		debug:      *debug,
		httpAddr:   *httpAddr,
		httpsAddr:  *httpsAddr,
		tunnelAddr: *tunnelAddr,
		tlsCrt:     *tlsCrt,
		tlsKey:     *tlsKey,
		clients:    *clients,
		logTo:      *logTo,
		logLevel:   *logLevel,
		version:    *version,
	}
}
