package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const usage1 string = `Usage: tunnel [OPTIONS] <command> [command args] [...]
options:
`

const usage2 string = `
Commands:
	tunnel start [tunnel] [...]    Start tunnels by name from config file
	tunnel start-all               Start all tunnels defined in config file
	tunnel list                    List tunnel names from config file

Examples:
	tunnel start www ssh
	tunnel -config=config.yaml -log=stdout -log-level 2 start ssh
	tunnel start-all

config.yaml:
	server_addr: SERVER_IP:4443
	insecure_skip_verify: true
	tunnels:
	  www:
	    proto: http
	    addr: http://IP:8080/ui/
	    auth: user:password
	    host: ui.mytunnel.com
	  ssh:
	    proto: tcp
	    addr: IP:22
	    remote_addr: 0.0.0.0:2222
`

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage1)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, usage2)
	}
}

type options struct {
	debug    bool
	config   string
	logTo    string
	logLevel int
	command  string
	args     []string
}

func parseArgs() (*options, error) {
	debug := flag.Bool("debug", false, "Starts gops agent")
	config := flag.String("config", filepath.Join(defaultPath(), "config.yaml"), "Path to tunnel configuration file")
	logTo := flag.String("log", "stdout", "Write log messages to this file, file name or 'stdout', 'stderr', 'none'")
	logLevel := flag.Int("log-level", 1, "Level of messages to log, 0-3")
	flag.Parse()

	opts := &options{
		debug:    *debug,
		config:   *config,
		logTo:    *logTo,
		logLevel: *logLevel,
		command:  flag.Arg(0),
	}

	switch opts.command {
	case "list":
		opts.args = flag.Args()[1:]
		if len(opts.args) > 0 {
			return nil, fmt.Errorf("list takes no arguments")
		}
	case "start":
		opts.args = flag.Args()[1:]
		if len(opts.args) == 0 {
			return nil, fmt.Errorf("you must specify at least one tunnel to start")
		}
	case "start-all":
		opts.args = flag.Args()[1:]
		if len(opts.args) > 0 {
			return nil, fmt.Errorf("start-all takes no arguments")
		}
	default:
		return nil, fmt.Errorf("expected command")
	}

	return opts, nil
}

func defaultPath() string {
	// user.Current() does not work on linux when cross compiling because
	// it requires CGO; use os.Getenv("HOME") hack until we compile natively
	var dir string

	if user, err := user.Current(); err == nil {
		dir = user.HomeDir
	} else {
		dir = os.Getenv("HOME")
	}

	return filepath.Join(dir, ".tunnel")
}
