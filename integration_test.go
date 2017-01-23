package tunnel_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/andrew-d/id"
	"github.com/mmatczuk/tunnel"
	"github.com/mmatczuk/tunnel/tunneltest"
)

const (
	payloadInitialSize = 512
	payloadLen         = 10
)

// testContext stores state shared between sub tests.
type testContext struct {
	// httpAddr is address for HTTP tests.
	httpAddr net.Addr
	// listener is address for TCP tests.
	tcpAddr net.Addr
	// payload is pre generated random data.
	payload [][]byte
}

var ctx testContext

func TestMain(m *testing.M) {
	// prepare server TCP listener
	serverTCPListener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer serverTCPListener.Close()

	// prepare tunnel server
	cert, id := selfSignedCert()
	s, err := tunnel.NewServer(&tunnel.ServerConfig{
		TLSConfig: tunneltest.TLSConfig(cert),
		AllowedClients: []*tunnel.AllowedClient{
			{
				ID:        id,
				Host:      "localhost",
				Listeners: []net.Listener{serverTCPListener},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	s.Start()
	defer s.Close()

	// run server HTTP interface
	serverHTTPListener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer serverHTTPListener.Close()
	go http.Serve(serverHTTPListener, s)

	// prepare local TCP echo service
	echoTCPListener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer echoTCPListener.Close()
	go tunneltest.EchoTCP(echoTCPListener)

	// prepare local HTTP echo service
	echoHTTPListener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer echoHTTPListener.Close()
	go tunneltest.EchoHTTP(echoHTTPListener)

	// prepare proxy
	httpproxy := tunnel.NewMultiHTTPProxy(map[string]*url.URL{
		"localhost:" + port(serverHTTPListener.Addr()): {
			Scheme: "http",
			Host:   echoHTTPListener.Addr().String(),
		},
	})
	tcpproxy := tunnel.NewMultiTCPProxy(map[string]string{
		port(serverTCPListener.Addr()): echoTCPListener.Addr().String(),
	})
	proxy := tunnel.Proxy(tunnel.ProxyFuncs{
		HTTP: httpproxy.Proxy,
		TCP:  tcpproxy.Proxy,
	})

	// prepare tunnel client
	c := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      s.Addr(),
		TLSClientConfig: tunneltest.TLSConfig(cert),
		Proxy:           proxy,
	})
	if err := c.Start(); err != nil {
		panic(err)
	}
	defer c.Stop()

	ctx.httpAddr = serverHTTPListener.Addr()
	ctx.tcpAddr = serverTCPListener.Addr()
	ctx.payload = randPayload(payloadInitialSize, payloadLen)

	m.Run()
}

func port(addr net.Addr) string {
	return fmt.Sprint(addr.(*net.TCPAddr).Port)
}

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
		panic("Read did not fill whole slice")
	}
	return b
}

func TestProxying(t *testing.T) {
	data := []struct {
		protocol string
		name     string
		seq      []uint
	}{
		{"http", "small", []uint{200, 160, 120, 80, 40, 20}},
		{"http", "mid", []uint{40, 80, 120, 160, 200}},
		{"http", "big", []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 200}},
		{"tcp", "small", []uint{200, 160, 120, 80, 40, 20}},
		{"tcp", "mid", []uint{40, 80, 120, 160, 200}},
		{"tcp", "big", []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 200}},
	}

	for _, tt := range data {
		tt := tt
		name := fmt.Sprintf("%s/%s", tt.protocol, tt.name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			switch tt.protocol {
			case "http":
				testHTTP(t, tt.seq)
			case "tcp":
				testTCP(t, tt.seq)
			default:
				panic("Unexpected network type")
			}
		})
	}
}

func testHTTP(t *testing.T, seq []uint) {
	for idx, s := range seq {
		for s > 0 {
			url := fmt.Sprintf("http://localhost:%s/some/path", port(ctx.httpAddr))
			r, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(ctx.payload[idx]))
			if err != nil {
				panic("Failed to create request")
			}
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				panic(fmt.Sprintf("HTTP error %s", err))
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

			n, m := len(b), len(ctx.payload[idx])
			if n != m {
				t.Error("Read mismatch", n, m)
			}
			s--
		}
	}
}

func testTCP(t *testing.T, seq []uint) {
	conn, err := net.Dial("tcp", ctx.tcpAddr.String())
	if err != nil {
		t.Fatal("Dial failed", err)
	}
	defer conn.Close()

	var buf = bigBuffer()
	var read, write int
	for idx, s := range seq {
		for s > 0 {
			m, err := conn.Write(ctx.payload[idx])
			if err != nil {
				t.Error("Write failed", err)
			}
			if m != len(ctx.payload[idx]) {
				t.Log("Write mismatch", m, len(ctx.payload[idx]))
			}
			write += m

			n, err := conn.Read(buf)
			if err != nil {
				t.Error("Read failed", err)
			}
			if n != m {
				t.Log("Read mismatch", n, m)
			}
			read += n
			s--
		}
	}

	for read < write {
		t.Log("No yet read everything", "write", write, "read", read)
		time.Sleep(10 * time.Millisecond)
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

func bigBuffer() []byte {
	return make([]byte, len(ctx.payload[len(ctx.payload)-1]))
}
