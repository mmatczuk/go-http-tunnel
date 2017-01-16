package tunnel

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mmatczuk/tunnel/mock"
)

func TestClient_Dial(t *testing.T) {
	s := httptest.NewTLSServer(nil)
	defer s.Close()

	c := NewClient(&ClientConfig{})

	addr := s.Listener.Addr().String()
	conn, err := c.dial("tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatal("Dial error", err)
	}
	if conn == nil {
		t.Fatal("Expected connection", err)
	}
	conn.Close()
}

func TestClient_DialBackoff(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := mock.NewMockBackoff(ctrl)
	gomock.InOrder(
		b.EXPECT().NextBackOff().Return(50*time.Millisecond).Times(2),
		b.EXPECT().NextBackOff().Return(-time.Millisecond),
	)

	d := func(network, addr string, config *tls.Config) (net.Conn, error) {
		return nil, errors.New("foobar")
	}

	c := NewClient(&ClientConfig{
		DialTLS: d,
		Backoff: b,
	})

	start := time.Now()
	_, err := c.dial("tcp", "8.8.8.8", nil)
	end := time.Now()

	if end.Sub(start) < 100*time.Millisecond {
		t.Fatal("Wait mismatch", err)
	}

	if err.Error() != "backoff limit exeded: foobar" {
		t.Fatal("Error mismatch", err)
	}
}
