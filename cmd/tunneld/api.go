// Copyright (C) 2021 Tribus Hannes
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	tunnel "github.com/hons82/go-http-tunnel"
	"github.com/hons82/go-http-tunnel/log"
)

// ApiConfig defines configuration for the API.
type ApiConfig struct {
	// Addr is TCP address to listen for client connections. If empty ":0" is used.
	Addr string
	//
	Server *tunnel.Server
	// Logger is optional logger. If nil logging is disabled.
	Logger log.Logger
}

func initAPIServer(config *ApiConfig) {

	logger := config.Logger
	if logger == nil {
		logger = log.NewNopLogger()
	}

	http.HandleFunc("/api/clients/list", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logger.Log(
				"level", 2,
				"action", "start client list",
			)
			info := config.Server.GetClientInfo()
			data, err := json.Marshal(info)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				e := fmt.Sprintf("Error on unmarshall item %s", err)
				w.Write([]byte(e))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			
			logger.Log(
				"level", 3,
				"action", "transferred",
				"bytes", len(data),
			)
		},
	))

	// Wrap our server with our gzip handler to gzip compress all responses.
	fatal("can not listen on: %s", http.ListenAndServe(config.Addr, nil))
}
