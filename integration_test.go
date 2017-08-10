package tunnel_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/proto"

	"golang.org/x/net/websocket"
)

const (
	payloadInitialSize = 512
	payloadLen         = 10
)

func echoHTTP(l net.Listener) {
	http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Body != nil {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				panic(err)
			}
			w.Write(body)
		}
	}))
}

func echoWS(l net.Listener) {
	wsServer := &websocket.Server{Handler: func(ws *websocket.Conn) {
		io.Copy(ws, ws)
	}}

	http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsServer.ServeHTTP(w, r)
	}))
}

func echoTCP(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			io.Copy(conn, conn)
		}()
	}
}

func makeEcho(t *testing.T) (http, ws, tcp net.Listener) {
	var err error

	// HTTP echo
	http, err = net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	go echoHTTP(http)

	// WS echo
	ws, err = net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	go echoWS(ws)

	// TCP echo
	tcp, err = net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	go echoTCP(tcp)

	return
}

func makeTunnelServer(t *testing.T) *tunnel.Server {
	cert, identifier := selfSignedCert()
	s, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:      ":0",
		TLSConfig: tlsConfig(cert),
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Subscribe(identifier)
	go s.Start()

	return s
}

func makeTunnelClient(t *testing.T, serverAddr string, tcpTunAddr, httpAddr, wsAddr, tcpAddr net.Addr) *tunnel.Client {
	httpProxy := tunnel.NewHTTPProxy(&url.URL{Scheme: "http", Host: httpAddr.String()}, nil)
	wsProxy := tunnel.NewWSProxy(&url.URL{Scheme: "ws", Host: wsAddr.String()}, nil)
	tcpProxy := tunnel.NewTCPProxy(tcpAddr.String(), nil)

	tunnels := map[string]*proto.Tunnel{
		proto.HTTP: {
			Protocol: proto.HTTP,
			Host:     "localhost",
			Auth:     "user:password",
		},
		// TODO WS tunnel?
		proto.TCP: {
			Protocol: proto.TCP,
			Addr:     tcpTunAddr.String(),
		},
	}

	cert, _ := selfSignedCert()
	c := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      serverAddr,
		TLSClientConfig: tlsConfig(cert),
		Tunnels:         tunnels,
		Proxy: tunnel.Proxy(tunnel.ProxyFuncs{
			HTTP: httpProxy.Proxy,
			WS:   wsProxy.Proxy,
			TCP:  tcpProxy.Proxy,
		}),
	})
	go c.Start()

	return c
}

func TestIntegration(t *testing.T) {
	// local services
	http, ws, tcp := makeEcho(t)
	defer http.Close()
	defer tcp.Close()

	// server
	s := makeTunnelServer(t)
	defer s.Stop()
	h := httptest.NewServer(s)
	defer h.Close()

	tcpTunAddr := freeAddr()

	// client
	c := makeTunnelClient(t, s.Addr(), tcpTunAddr,
		http.Addr(), ws.Addr(), tcp.Addr(),
	)
	// FIXME: replace sleep with client state change watch when ready
	time.Sleep(500 * time.Millisecond)
	defer c.Stop()

	payload := randPayload(payloadInitialSize, payloadLen)
	table := []struct {
		S []uint
	}{
		{[]uint{200, 160, 120, 80, 40, 20}},
		{[]uint{40, 80, 120, 160, 200}},
		{[]uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 200}},
	}

	var wg sync.WaitGroup
	for _, test := range table {
		for i, repeat := range test.S {
			p := payload[i]
			r := repeat

			wg.Add(1)
			go func() {
				testHTTP(t, h.Listener.Addr(), p, r)
				wg.Done()
			}()
			wg.Add(1)
			go func() {
				config, err := websocket.NewConfig(
					fmt.Sprintf("ws://localhost:%s/some/path", port(h.Listener.Addr())),
					fmt.Sprintf("http://localhost:%s/", port(h.Listener.Addr())),
				)
				if err != nil {
					panic("Invalid config")
				}
				config.Header.Set("Authorization", "Basic dXNlcjpwYXNzd29yZA==")

				ws, err := websocket.DialConfig(config)
				if err != nil {
					t.Fatal("Dial failed", err)
				}
				defer ws.Close()
				testConn(t, ws, p, r)
				wg.Done()
			}()
			wg.Add(1)
			go func() {
				conn, err := net.Dial("tcp", tcpTunAddr.String())
				if err != nil {
					t.Fatal("Dial failed", err)
				}
				defer conn.Close()
				testConn(t, conn, p, r)
				wg.Done()
			}()
		}
	}
	wg.Wait()
}

func testHTTP(t *testing.T, addr net.Addr, payload []byte, repeat uint) {
	url := fmt.Sprintf("http://localhost:%s/some/path", port(addr))

	for repeat > 0 {
		r, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			t.Fatal("Failed to create request")
		}
		r.SetBasicAuth("user", "password")

		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Error(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Error("Unexpected status code", resp)
		}
		if resp.Body == nil {
			t.Error("No body")
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error("Read error")
		}
		n, m := len(b), len(payload)
		if n != m {
			t.Error("Write read mismatch", n, m)
		}
		repeat--
	}
}

func testConn(t *testing.T, conn net.Conn, payload []byte, repeat uint) {
	var buf = make([]byte, 10*1024*1024)
	var read, write int
	for repeat > 0 {
		m, err := conn.Write(payload)
		if err != nil {
			t.Error("Write failed", err)
		}
		if m != len(payload) {
			t.Log("Write mismatch", m, len(payload))
		}
		write += m

		n, err := conn.Read(buf)
		if err != nil {
			t.Error("Read failed", err)
		}
		read += n
		repeat--
	}

	for read < write {
		t.Log("No yet read everything", "write", write, "read", read)
		time.Sleep(50 * time.Millisecond)
		n, err := conn.Read(buf)
		if err != nil {
			t.Error("Read failed", err)
		}
		read += n
	}

	if read != write {
		t.Fatal("Write read mismatch", read, write)
	}
}

//
// helpers
//

// randPayload returns slice of randomly initialised data buffers.
func randPayload(initialSize, n int) [][]byte {
	payload := make([][]byte, n)
	l := initialSize
	for i := 0; i < n; i++ {
		payload[i] = randBytes(l)
		l *= 2
	}
	return payload
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	read, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	if read != n {
		panic("read did not fill whole slice")
	}
	return b
}

func freeAddr() net.Addr {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr()
}

func port(addr net.Addr) string {
	return fmt.Sprint(addr.(*net.TCPAddr).Port)
}

func selfSignedCert() (tls.Certificate, id.ID) {
	cert, err := tls.LoadX509KeyPair("./test-fixtures/selfsigned.crt", "./test-fixtures/selfsigned.key")
	if err != nil {
		panic(err)
	}
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		panic(err)
	}

	return cert, id.New(x509Cert.Raw)
}

func tlsConfig(cert tls.Certificate) *tls.Config {
	c := &tls.Config{
		Certificates:             []tls.Certificate{cert},
		ClientAuth:               tls.RequestClientCert,
		SessionTicketsDisabled:   true,
		InsecureSkipVerify:       true,
		MinVersion:               tls.VersionTLS12,
		CipherSuites:             []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}
	c.BuildNameToCertificate()
	return c
}
