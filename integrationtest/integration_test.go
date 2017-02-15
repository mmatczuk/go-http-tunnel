package integrationtest

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/mmatczuk/go-http-tunnel"
	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/log"
	"github.com/mmatczuk/go-http-tunnel/proto"
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
	logger := log.NewFilterLogger(log.NewStdLogger(), 1)

	// prepare tunnel server
	cert, identifier := selfSignedCert()
	s, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:      ":0",
		TLSConfig: TLSConfig(cert),
		Logger:    log.NewContext(logger).WithPrefix("server", ":"),
	})
	if err != nil {
		panic(err)
	}
	s.Subscribe(identifier)
	go s.Start()
	defer s.Stop()

	// run server HTTP interface
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	go http.Serve(l, s)

	httpAddr := l.Addr()

	// prepare local TCP echo service
	echoTCPListener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer echoTCPListener.Close()
	go EchoTCP(echoTCPListener)

	// prepare local HTTP echo service
	echoHTTPListener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer echoHTTPListener.Close()
	go EchoHTTP(echoHTTPListener)

	// allocate free port
	tcpAddr := freeAddr()

	// prepare tunnel client
	tunnels := map[string]*proto.Tunnel{
		"http": {
			Protocol: proto.HTTP,
			Host:     "localhost",
			Auth:     "user:password",
		},
		"tcp": {
			Protocol: proto.TCP,
			Addr:     tcpAddr.String(),
		},
	}

	httpProxy := tunnel.NewMultiHTTPProxy(map[string]*url.URL{
		"localhost:" + port(httpAddr): {
			Scheme: "http",
			Host:   "127.0.0.1:" + port(echoHTTPListener.Addr()),
		},
	}, log.NewContext(logger).WithPrefix("HTTP proxy", ":"))

	tcpProxy := tunnel.NewMultiTCPProxy(map[string]string{
		port(tcpAddr): echoTCPListener.Addr().String(),
	}, log.NewContext(logger).WithPrefix("TCP proxy", ":"))

	c := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      s.Addr(),
		TLSClientConfig: TLSConfig(cert),
		Tunnels:         tunnels,
		Proxy: tunnel.Proxy(tunnel.ProxyFuncs{
			HTTP: httpProxy.Proxy,
			TCP:  tcpProxy.Proxy,
		}),
		Logger: log.NewContext(logger).WithPrefix("client", ":"),
	})
	go c.Start()
	// FIXME: replace sleep with client state change watch when ready
	time.Sleep(500 * time.Millisecond)
	defer c.Stop()

	ctx.httpAddr = httpAddr
	ctx.tcpAddr = tcpAddr
	ctx.payload = randPayload(payloadInitialSize, payloadLen)

	m.Run()
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
		for idx, repeat := range tt.seq {
			name := fmt.Sprintf("%s/%s/%d", tt.protocol, tt.name, idx)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				switch tt.protocol {
				case "http":
					testHTTP(t, ctx.payload[idx], repeat)
				case "tcp":
					testTCP(t, ctx.payload[idx], repeat)
				default:
					panic("Unexpected network type")
				}
			})
		}
	}
}

func testHTTP(t *testing.T, payload []byte, repeat uint) {
	for repeat > 0 {
		url := fmt.Sprintf("http://localhost:%s/some/path", port(ctx.httpAddr))
		r, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			panic("Failed to create request")
		}
		r.SetBasicAuth("user", "password")

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

		n, m := len(b), len(payload)
		if n != m {
			t.Error("Write read mismatch", n, m)
		}
		repeat--
	}
}

func testTCP(t *testing.T, payload []byte, repeat uint) {
	conn, err := net.Dial("tcp", ctx.tcpAddr.String())
	if err != nil {
		t.Fatal("Dial failed", err)
	}
	defer conn.Close()

	var buf = bigBuffer()
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

func bigBuffer() []byte {
	return make([]byte, len(ctx.payload[len(ctx.payload)-1]))
}

func randPayload(initialSize, n int) [][]byte {
	payload := make([][]byte, n)
	l := initialSize
	for i := 0; i < n; i++ {
		payload[i] = RandBytes(l)
		l *= 2
	}
	return payload
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
