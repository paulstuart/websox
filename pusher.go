// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = time.Second

	// LogFlags are the default log flags
	LogFlags = log.Ldate | log.Lmicroseconds | log.Lshortfile
)

var (
	mu      sync.Mutex
	counter int
)

func pusherID() string {
	mu.Lock()
	counter++
	id := fmt.Sprintf("id:%03d ", counter)
	mu.Unlock()
	return id
}

// Setup returns a channel to get data and one that return an error value and optional artifacts
// from the results of that action
//
// If the setup function cannot do the required processing,
// it should return a nil interface channel and send an error message in the error channel
//
// The Results channel is closed by Pusher() when it is done processing (due to session timeout or error)
type Setup func() (chan io.Reader, chan Results)

// Pusher gets send/recv channels from the setup function
// and sets up the environment for bringing up an event loop on the websocket connection
func Pusher(setup Setup, expires, pingFreq time.Duration, contacted func(), logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if logger == nil {
			logger = log.New(os.Stderr, pusherID(), LogFlags)
		}

		// getter gets data to be sent
		// teller returns results of what was sent
		getter, teller := setup()
		if getter == nil {
			results := <-teller
			logger.Println("closing teller - app startup failure")
			close(teller)
			logger.Println("Pusher setup error:", results.ErrMsg)
			http.Error(w, results.ErrMsg, http.StatusInternalServerError)
			return
		}

		upgrader := websocket.Upgrader{} // use default options
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Print("push upgrade error:", err)
			return
		}

		// optional monitoring of activity
		if contacted != nil {
			contacted()
			conn.SetPongHandler(func(s string) error {
				contacted()
				return nil
			})
		}

		//response := make(chan io.Reader)
		//src := make(chan io.Reader)
		//src, response := eventloop(getter, teller, expires, logger)

		// listen for messages from client
		listener(conn, getter, teller, pingFreq, expires, contacted, logger)
		//listener(conn, logger, src, response, pingFreq, contacted)

		// event loop for all other i/o
		//ioloop(conn, getter, teller, response, expires, pingFreq, logger)
	}
}

func ping(conn *websocket.Conn) error {
	now := time.Now()
	err := conn.WriteControl(websocket.PingMessage, []byte(now.String()), now.Add(writeWait))
	if err != nil && err != websocket.ErrCloseSent {
		if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		}
	}
	return err
}

// handle incoming messages
// no concurrent access for conn, so all is controlled here
func listener(
	conn *websocket.Conn,
	src chan io.Reader,
	response chan Results,
	pingFreq, expires time.Duration,
	contacted func(),
	logger *log.Logger,
) {

	defer func() {
		close(response)
		logger.Println("websocket server closing")
		msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		if err := conn.WriteMessage(websocket.CloseMessage, msg); err != nil {
			logger.Println("websocket server close message error:", err)
		}
		conn.Close()
	}()

	var expired <-chan time.Time
	if expires != 0 {
		expired = time.NewTimer(expires).C
	}

	for {
		logger.Println("waiting for input")

		ticker := time.NewTicker(pingFreq)
		select {
		case <-expired:
			logger.Println("session expired")
			return
		case <-ticker.C:
			if err := ping(conn); err != nil {
				logger.Println("ping error:", err)
				return
			}
		case r, ok := <-src:
			if !ok {
				logger.Println("src closed")
				return
			}

			// send our message
			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				logger.Println("writer error:", err)
				return
			}

			if _, err = io.Copy(w, r); err != nil {
				logger.Println("copy error:", err)
				return
			}

			if err := w.Close(); err == nil {
				logger.Println("close error:", err)
			}

			logger.Println("waiting for reader")
			messageType, r, err := conn.NextReader()
			if err != nil {
				logger.Println("listener read error:", err)
				return
			}
			logger.Println("we have a reply")
			if contacted != nil {
				contacted()
			}

			switch messageType {
			case websocket.TextMessage:
				var results Results
				if err := json.NewDecoder(r).Decode(&results); err != nil {
					logger.Println("status json error:", err)
					results = Results{ErrMsg: err.Error()}
				}
				response <- results
			case websocket.CloseMessage:
				logger.Println("uhoh! closing time!")
				return
			}
		}

	}
}
