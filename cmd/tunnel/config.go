package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/mmatczuk/go-http-tunnel/proto"
)

type BackoffConfig struct {
	InitialInterval time.Duration `yaml:"interval,omitempty"`
	Multiplier      float64       `yaml:"multiplier,omitempty"`
	MaxInterval     time.Duration `yaml:"max_interval,omitempty"`
	MaxElapsedTime  time.Duration `yaml:"max_time,omitempty"`
}

type TunnelConfig struct {
	Protocol   string `yaml:"proto,omitempty"`
	Addr       string `yaml:"addr,omitempty"`
	Auth       string `yaml:"auth,omitempty"`
	Host       string `yaml:"host,omitempty"`
	RemoteAddr string `yaml:"remote_addr,omitempty"`
}

type Config struct {
	ServerAddr         string                   `yaml:"server_addr,omitempty"`
	InsecureSkipVerify bool                     `yaml:"insecure_skip_verify,omitempty"`
	TLSCrt             string                   `yaml:"tls_crt,omitempty"`
	TLSKey             string                   `yaml:"tls_key,omitempty"`
	Backoff            *BackoffConfig           `yaml:"backoff,omitempty"`
	Tunnels            map[string]*TunnelConfig `yaml:"tunnels,omitempty"`
}

var defaultBackoffConfig = BackoffConfig{
	InitialInterval: 500 * time.Millisecond,
	Multiplier:      1.5,
	MaxInterval:     60 * time.Second,
	MaxElapsedTime:  15 * time.Minute,
}

func loadConfiguration(path string) (*Config, error) {
	configBuf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %s", path, err)
	}

	// deserialize/parse the config
	var config Config
	if err = yaml.Unmarshal(configBuf, &config); err != nil {
		return nil, fmt.Errorf("failed to parse file %q: %s", path, err)
	}

	// set default values
	if config.TLSCrt == "" {
		config.TLSCrt = filepath.Join(filepath.Dir(path), "client.crt")
	}
	if config.TLSKey == "" {
		config.TLSKey = filepath.Join(filepath.Dir(path), "client.key")
	}

	if config.Backoff == nil {
		config.Backoff = &defaultBackoffConfig
	} else {
		if config.Backoff.InitialInterval == 0 {
			config.Backoff.InitialInterval = defaultBackoffConfig.InitialInterval
		}
		if config.Backoff.Multiplier == 0 {
			config.Backoff.Multiplier = defaultBackoffConfig.Multiplier
		}
		if config.Backoff.MaxInterval == 0 {
			config.Backoff.MaxInterval = defaultBackoffConfig.MaxInterval
		}
		if config.Backoff.MaxElapsedTime == 0 {
			config.Backoff.MaxElapsedTime = defaultBackoffConfig.MaxElapsedTime
		}
	}

	// validate and normalize configuration
	if config.ServerAddr == "" {
		return nil, fmt.Errorf("server_addr: missing")
	}

	if config.ServerAddr, err = normalizeAddress(config.ServerAddr); err != nil {
		return nil, fmt.Errorf("server_addr: %s", err)
	}

	for name, t := range config.Tunnels {
		switch t.Protocol {
		case proto.HTTP:
			if err := validateHTTP(t); err != nil {
				return nil, fmt.Errorf("%s %s", name, err)
			}
		case proto.TCP, proto.TCP4, proto.TCP6:
			if err := validateTCP(t); err != nil {
				return nil, fmt.Errorf("%s %s", name, err)
			}
		default:
			return nil, fmt.Errorf("%s invalid protocol %q", name, t.Protocol)
		}
	}

	return &config, nil
}

func validateHTTP(t *TunnelConfig) error {
	var err error
	if t.Host == "" {
		return fmt.Errorf("host: missing")
	}
	if t.Addr == "" {
		return fmt.Errorf("addr: missing")
	}
	if t.Addr, err = normalizeURL(t.Addr); err != nil {
		return fmt.Errorf("addr: %s", err)
	}

	// unexpected

	if t.RemoteAddr != "" {
		return fmt.Errorf("remote_addr: unexpected")
	}

	return nil
}

func validateTCP(t *TunnelConfig) error {
	var err error
	if t.RemoteAddr, err = normalizeAddress(t.RemoteAddr); err != nil {
		return fmt.Errorf("remote_addr: %s", err)
	}
	if t.Addr == "" {
		return fmt.Errorf("addr: missing")
	}
	if t.Addr, err = normalizeAddress(t.Addr); err != nil {
		return fmt.Errorf("addr: %s", err)
	}

	// unexpected

	if t.Host != "" {
		return fmt.Errorf("host: unexpected")
	}
	if t.Auth != "" {
		return fmt.Errorf("auth: unexpected")
	}

	return nil
}
