// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"fmt"
	"net"
)

func keepAlive(conn net.Conn) error {
	c, ok := conn.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("Bad connection type: %T", c)
	}

	if err := c.SetKeepAlive(true); err != nil {
		return err
	}

	return nil
}
