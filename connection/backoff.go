// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package connection

import "time"

// Backoff defines behavior of staggering reconnection retries.
type Backoff interface {
	// Next returns the duration to sleep before retrying to reconnect.
	// If the returned value is negative, the retry is aborted.
	NextBackOff() time.Duration

	// Reset is used to signal a reconnection was successful and next
	// call to Next should return desired time duration for 1st reconnection
	// attempt.
	Reset()
}

func NewDefaultBackoffConfig() *BackoffConfig {
	return &BackoffConfig{
		Interval:    DefaultBackoffInterval,
		Multiplier:  DefaultBackoffMultiplier,
		MaxInterval: DefaultBackoffMaxInterval,
		MaxTime:     DefaultBackoffMaxTime,
	}
}

// Default backoff configuration.
const (
	DefaultBackoffInterval    = 500 * time.Millisecond
	DefaultBackoffMultiplier  = 1.5
	DefaultBackoffMaxInterval = 60 * time.Second
	DefaultBackoffMaxTime     = 15 * time.Minute
)

// BackoffConfig defines behavior of staggering reconnection retries.
type BackoffConfig struct {
	Interval    time.Duration `yaml:"interval"`
	Multiplier  float64       `yaml:"multiplier"`
	MaxInterval time.Duration `yaml:"max_interval"`
	MaxTime     time.Duration `yaml:"max_time"`
}
