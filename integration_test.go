package tunnel_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andrew-d/id"
	"github.com/mmatczuk/tunnel"
	"github.com/mmatczuk/tunnel/tunneltest"
)

const (
	payloadInitialSize = 32
	payloadLen         = 10
)

// testContext stores state shared between sub tests.
type testContext struct {
	// handler is entry point for HTTP tests.
	handler http.Handler
	// listener is entry point for TCP tests.
	listener net.Listener
	// payload is pre generated random data.
	payload [][]byte
}

var ctx testContext

func TestMain(m *testing.M) {
	cert, id := selfSignedCert()

	// prepare TCP listener
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer l.Close()

	// prepare tunnel server
	s, err := tunnel.NewServer(&tunnel.ServerConfig{
		TLSConfig: tunneltest.TLSConfig(cert),
		AllowedClients: []*tunnel.AllowedClient{
			{
				ID:        id,
				Host:      "foobar.com",
				Listeners: []net.Listener{l},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	s.Start()
	defer s.Close()

	// prepare tunnel client
	c := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      s.Addr(),
		TLSClientConfig: tunneltest.TLSConfig(cert),
		Proxy:           tunneltest.EchoProxyFunc,
	})
	if err := c.Start(); err != nil {
		panic(err)
	}
	defer c.Stop()

	ctx.handler = s
	ctx.listener = l
	ctx.payload = randPayload(payloadInitialSize, payloadLen)

	m.Run()
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
		{"http", "small", []uint{100, 80, 60, 40, 20, 10}},
		{"http", "mid", []uint{20, 40, 60, 80, 100}},
		{"http", "big", []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 100}},
		{"tcp", "small", []uint{100, 80, 60, 40, 20, 10}},
		{"tcp", "mid", []uint{20, 40, 60, 80, 100}},
		{"tcp", "big", []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 100}},
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
	var buf = bytes.NewBuffer(bigBuffer())
	for idx, s := range seq {
		for s > 0 {
			r, err := http.NewRequest(http.MethodPost, "http://foobar.com/some/path", bytes.NewReader(ctx.payload[idx]))
			if err != nil {
				panic("Failed to create request")
			}
			buf.Reset()
			w := &httptest.ResponseRecorder{
				HeaderMap: make(http.Header),
				Body:      buf,
				Code:      200,
			}
			ctx.handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Error("Unexpected status code", w)
			}
			n, m := w.Body.Len(), len(ctx.payload[idx])
			if n != m {
				t.Log("Read mismatch", n, m)
			}
			s--
		}
	}
}

func testTCP(t *testing.T, seq []uint) {
	conn, err := net.Dial("tcp", ctx.listener.Addr().String())
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
