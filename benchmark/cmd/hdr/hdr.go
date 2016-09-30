package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/codahale/hdrhistogram"
	"github.com/koding/logging"
	"github.com/koding/multiconfig"
)

type config struct {
	Rate int64
}

type event struct {
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
	Latency   int64     `json:"latency"`
	BytesOut  int64     `json:"bytes_out"`
	BytesIn   int64     `json:"bytes_in"`
	Error     string    `json:"error"`
}

func main() {
	m := multiconfig.New()
	config := new(config)
	m.MustLoad(config)
	m.MustValidate(config)

	interval := int64(0)
	if config.Rate != 0 {
		interval = int64(time.Second) / config.Rate
	}

	hist := hdrhistogram.New(int64(0), int64(30*time.Second), 5)
	r := bufio.NewReader(os.Stdin)

	var errorCount = 0
	var e event
	for {
		l, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			logging.Fatal("Read error:%s", err)
		}
		if err = json.Unmarshal(l, &e); err != nil {
			logging.Fatal("Parse error:%s", err)
		}

		if e.Error != "" {
			errorCount++
			continue
		}

		if interval != 0 {
			hist.RecordCorrectedValue(e.Latency, interval)
		} else {
			hist.RecordValue(e.Latency)
		}
	}

	for _, b := range hist.CumulativeDistribution() {
		v := float64(b.ValueAt)
		p := b.Quantile

		var inv float64
		if 100-p == 0 {
			inv = math.MaxFloat64
		} else {
			inv = float64(100) / float64(100-p)
		}
		os.Stdout.WriteString(fmt.Sprintf("%f %f %d %f\n", v/1000/1000, p/100, b.Count, inv))
	}

	if errorCount > 0 {
		os.Stderr.WriteString(fmt.Sprintf("Skipped %d error responses", errorCount))
	}
}
