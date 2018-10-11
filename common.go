package tunnel

import (
	"net"

	"github.com/mmatczuk/go-http-tunnel/id"
	"github.com/mmatczuk/go-http-tunnel/log"
)

const HeaderConnectionsCount = "X-Connections-Count"

type clientConnectionController struct {
	cfg    *RegisteredClientConfig
	conn   net.Conn
	ID     id.ID
	logger log.Logger
}
