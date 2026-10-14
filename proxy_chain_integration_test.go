// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel_test

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
)

func TestProxyChain_HTTPAndCONNECT(t *testing.T) {
	// Remote HTTP target simulating external internet server
	remoteHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Remote-Echo", "ok")
		fmt.Fprintf(w, "remote response for path: %s", r.URL.Path)
	}))
	defer remoteHTTP.Close()

	// Remote TCP target simulating external TCP/HTTPS server
	remoteTCP, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start remote TCP server: %v", err)
	}
	defer remoteTCP.Close()

	go func() {
		for {
			conn, err := remoteTCP.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Tunnel server (tunneld)
	server := makeTunnelServer(t)
	defer server.Stop()

	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	// Tunnel client configured as forward proxy chain kit
	httpProxy := tunnel.NewMultiHTTPProxy(map[string]*url.URL{}, log.NewStdLogger())
	httpProxy.ForwardProxy = true

	tcpProxy := tunnel.NewMultiTCPProxy(map[string]string{}, log.NewStdLogger())
	tcpProxy.ForwardProxy = true

	tunnels := map[string]*proto.Tunnel{
		"chain": {
			Protocol: proto.HTTP,
			Host:     "*",
			Auth:     "proxyuser:proxypass",
		},
	}

	client, err := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      server.Addr(),
		TLSClientConfig: tlsConfig(),
		Tunnels:         tunnels,
		Proxy: tunnel.Proxy(tunnel.ProxyFuncs{
			HTTP: httpProxy.Proxy,
			TCP:  tcpProxy.Proxy,
		}),
		Logger: log.NewStdLogger(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Stop()

	go func() {
		if err := client.Start(); err != nil {
			t.Log(err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	proxyAddr := httpServer.Listener.Addr().String()

	t.Run("HTTP forward proxy missing auth", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, remoteHTTP.URL+"/test-unauth", nil)
		if err != nil {
			t.Fatal(err)
		}

		proxyURL, _ := url.Parse("http://" + proxyAddr)
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		httpClient := &http.Client{Transport: transport}

		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("proxy request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusProxyAuthRequired && resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected proxy auth required, got status %d", resp.StatusCode)
		}
	})

	t.Run("HTTP forward proxy success", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, remoteHTTP.URL+"/hello-chain", nil)
		if err != nil {
			t.Fatal(err)
		}
		authVal := base64.StdEncoding.EncodeToString([]byte("proxyuser:proxypass"))
		req.Header.Set("Proxy-Authorization", "Basic "+authVal)

		proxyURL, _ := url.Parse("http://" + proxyAddr)
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		httpClient := &http.Client{Transport: transport}

		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("proxy request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got: %s", resp.Status)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		expectedBody := "remote response for path: /hello-chain"
		if string(body) != expectedBody {
			t.Fatalf("expected body %q, got %q", expectedBody, string(body))
		}
		if resp.Header.Get("X-Remote-Echo") != "ok" {
			t.Fatalf("expected remote header, got: %v", resp.Header)
		}
	})

	t.Run("CONNECT method TCP tunnel success", func(t *testing.T) {
		conn, err := net.Dial("tcp", proxyAddr)
		if err != nil {
			t.Fatalf("failed to dial tunneld proxy: %v", err)
		}
		defer conn.Close()

		authVal := base64.StdEncoding.EncodeToString([]byte("proxyuser:proxypass"))
		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\n\r\n",
			remoteTCP.Addr().String(), remoteTCP.Addr().String(), authVal)

		if _, err := conn.Write([]byte(connectReq)); err != nil {
			t.Fatalf("failed to send CONNECT request: %v", err)
		}

		reader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
		if err != nil {
			t.Fatalf("failed to read CONNECT response: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 Connection Established, got %s", resp.Status)
		}

		// Exchange TCP stream payload over established tunnel
		payload := []byte("ping pong tcp over proxy chain")
		if _, err := conn.Write(payload); err != nil {
			t.Fatalf("failed to write payload: %v", err)
		}

		recvBuf := make([]byte, len(payload))
		if _, err := io.ReadFull(reader, recvBuf); err != nil {
			t.Fatalf("failed to read payload from echo server: %v", err)
		}

		if !strings.EqualFold(string(recvBuf), string(payload)) {
			t.Fatalf("payload mismatch: expected %q, got %q", string(payload), string(recvBuf))
		}
	})
}
