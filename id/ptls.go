// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package id

import (
	"crypto/tls"
	"fmt"
)

var emptyID [32]byte

// PeerID is modified https://github.com/andrew-d/ptls/blob/b89c7dcc94630a77f225a48befd3710144c7c10e/ptls.go#L81
func PeerID(conn *tls.Conn) (ID, error) {
	// Try a TLS connection over the given connection. We explicitly perform
	// the handshake, since we want to maintain the invariant that, if this
	// function returns successfully, then the connection should be valid
	// and verified.
	if err := conn.Handshake(); err != nil {
		return emptyID, err
	}

	cs := conn.ConnectionState()

	// We should have exactly one peer certificate.
	certs := cs.PeerCertificates
	if cl := len(certs); cl != 1 {
		return emptyID, ImproperCertsNumberError{cl}
	}

	// Get remote cert's ID.
	remoteCert := certs[0]
	remoteID := New(remoteCert.Raw)

	return remoteID, nil
}

// ImproperCertsNumberError is returned from Server/Client whenever the remote
// peer presents a number of PeerCertificates that is not 1.
type ImproperCertsNumberError struct {
	n int
}

func (e ImproperCertsNumberError) Error() string {
	return fmt.Sprintf("ptls: expecting 1 peer certificate, got %d", e.n)
}
