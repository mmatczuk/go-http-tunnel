package h2tun_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/andrew-d/id"
	"github.com/koding/h2tun"
	"github.com/koding/h2tun/h2tuntest"
)

const (
	payloadInitialSize = 16
	payloadLen         = 10
)

var payload = randPayload(payloadInitialSize, payloadLen)

func TestProxying(t *testing.T) {
	t.Parallel()

	cert, id := selfSignedCert()

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Error("Listen failed", err)
	}
	defer l.Close()

	s, err := h2tun.NewServer(&h2tun.ServerConfig{
		TLSConfig: h2tuntest.TLSConfig(cert),
		AllowedClients: []*h2tun.AllowedClient{
			{
				ID:        id,
				Host:      "foobar.com",
				Listeners: []net.Listener{l},
			},
		},
	})
	if err != nil {
		t.Error("Server creation failed", err)
	}
	s.Start()
	defer s.Close()

	c := h2tun.NewClient(&h2tun.ClientConfig{
		ServerAddr:      s.Addr(),
		TLSClientConfig: h2tuntest.TLSConfig(cert),
		Proxy:           h2tuntest.EchoProxyFunc,
	})
	if err := c.Connect(); err != nil {
		t.Error("Client start failed", err)
	}
	defer c.Close()

	data := []struct {
		protocol string
		repeat   int
		seq      []uint
	}{
		{"http", 16, []uint{1000, 800, 600, 400, 200, 100}},
		{"http", 8, []uint{200, 400, 600, 800, 1000}},
		{"http", 4, []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 1000}},

		{"tcp", 16, []uint{1000, 800, 600, 400, 200, 100}},
		{"tcp", 8, []uint{200, 400, 600, 800, 1000}},
		{"tcp", 4, []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 1000}},
	}

	var wg sync.WaitGroup
	for _, tt := range data {
		for i := 0; i < tt.repeat; i++ {
			wg.Add(1)
			switch tt.protocol {
			case "http":
				go testHTTP(t, s, tt.seq, &wg)
			case "tcp":
				go testTCP(t, l.Addr().String(), tt.seq, &wg)
			default:
				panic("Unexpected network type")
			}
		}
	}
	wg.Wait()
}

func testHTTP(t *testing.T, h http.Handler, seq []uint, wg *sync.WaitGroup) {
	defer wg.Done()

	var buf = bytes.NewBuffer(bigBuffer())
	for idx, s := range seq {
		for s > 0 {
			r, err := http.NewRequest(http.MethodPost, "http://foobar.com/some/path", bytes.NewReader(payload[idx]))
			if err != nil {
				panic("Failed to create request")
			}
			buf.Reset()
			w := &httptest.ResponseRecorder{
				HeaderMap: make(http.Header),
				Body:      buf,
				Code:      200,
			}
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Error("Unexpected status code", w)
			}
			n, m := w.Body.Len(), len(payload[idx])
			if n != m {
				t.Log("Read mismatch", n, m)
			}
			s--
		}
	}
}

func testTCP(t *testing.T, addr string, seq []uint, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Error("Dial failed", err)
	}
	defer conn.Close()

	var buf = bigBuffer()
	var read, write int
	for idx, s := range seq {
		for s > 0 {
			m, err := conn.Write(payload[idx])
			if err != nil {
				t.Error("Write failed", err)
			}
			if m != len(payload[idx]) {
				t.Log("Write mismatch", m, len(payload[idx]))
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

func bigBuffer() []byte {
	return make([]byte, len(payload[len(payload)-1]))
}
