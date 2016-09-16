package h2tun_test

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func sleep() {
	time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type EchoMessage struct {
	Value string `json:"value,omitempty"`
	Close bool   `json:"close,omitempty"`
}

func handlerEchoWS(sleepFn func()) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) (e error) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		defer func() {
			err := conn.Close()
			if e == nil {
				e = err
			}
		}()

		if sleepFn != nil {
			sleepFn()
		}

		for {
			var msg EchoMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				return fmt.Errorf("ReadJSON error: %s", err)
			}

			if sleepFn != nil {
				sleepFn()
			}

			err = conn.WriteJSON(&msg)
			if err != nil {
				return fmt.Errorf("WriteJSON error: %s", err)
			}

			if msg.Close {
				return nil
			}
		}
	}
}

func handlerEchoHTTP(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, r.URL.Query().Get("echo"))
}

func handlerLatencyEchoHTTP(w http.ResponseWriter, r *http.Request) {
	sleep()
	handlerEchoHTTP(w, r)
}

func handlerEchoTCP(conn net.Conn) {
	io.Copy(conn, conn)
}

func handlerLatencyEchoTCP(conn net.Conn) {
	sleep()
	handlerEchoTCP(conn)
}
