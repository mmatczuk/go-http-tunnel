package tunnel

import (
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mmatczuk/tunnel/mock"
)

func TestClient_Backoff(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := mock.NewMockBackoff(ctrl)
	gomock.InOrder(
		b.EXPECT().NextBackOff().Return(time.Millisecond).Times(2),
		b.EXPECT().NextBackOff().Return(-time.Millisecond),
	)

	d := func(network, addr string, config *tls.Config) (net.Conn, error) {
		return nil, errors.New("foobar")
	}

	c := NewClient(&ClientConfig{
		ServerAddr: "8.8.8.8",
		DialTLS:    d,
		Backoff:    b,
	})

	err := c.Start()
	if !strings.Contains(err.Error(), "backoff limit exeded: foobar") {
		t.Fatal("Error mismatch", err)
	}
}
