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

// Setup returns channels to get data and return the error when trying to save said data
// If the setup function cannot do the required processing,
// it should return a nil interface channel and send an error message in the error channel
//
// The Results channel is closed by Pusher() when it is done processing (due to session timeout or error)
type Setup func() (chan io.Reader, chan Results)

// Pusher gets send/recv channels from the setup function
// and apply the channel data to a websocket connection
func Pusher(setup Setup, expires, pingFreq time.Duration, contacted func(), logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if logger == nil {
			logger = log.New(os.Stderr, pusherID(), LogFlags)
		}
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

		if contacted != nil {
			contacted()
		}

		closeHandler := conn.CloseHandler()
		conn.SetCloseHandler(func(code int, text string) error {
			logger.Printf("got close code: %d text: %s\n", code, text)
			if closeHandler != nil {
				logger.Println("calling original closeHandler")
				return closeHandler(code, text)
			}
			return nil
		})

		if contacted != nil {
			conn.SetPongHandler(func(s string) error {
				contacted()
				return nil
			})
		}

		ticker := time.NewTicker(pingFreq)
		ping := func(now time.Time) error {
			err := conn.WriteControl(websocket.PingMessage, []byte(now.String()), now.Add(writeWait))
			if err == websocket.ErrCloseSent {
				return nil
			} else if e, ok := err.(net.Error); ok && e.Temporary() {
				return nil
			}
			return err
		}

		// copy the message to the websocket connection
		send := func(r io.Reader) error {

			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return err
			}

			_, err = io.Copy(w, r)

			if err2 := w.Close(); err == nil {
				err = err2
			}

			return err
		}

		// listen for messages from client
		response := make(chan io.Reader)
		go func() {
			for {
				logger.Println("waiting for reply")
				messageType, r, err := conn.NextReader()
				if err != nil {
					logger.Println("pusher read error:", err)
					break
				}
				logger.Println("we have a reply")
				if contacted != nil {
					contacted()
				}

				switch messageType {
				case websocket.TextMessage:
					response <- r
				case websocket.CloseMessage:
					logger.Println("uhoh! closing time!")
					break
				}

			}
			logger.Println("====> read loop complete")
			close(response)
		}()

		expired := make(<-chan time.Time)
		if expires != 0 {
			expired = time.NewTimer(expires).C
		}

		active, open := false, true

	loop:
		for open || active {
			select {
			case <-expired:
				logger.Println("session has expired")
				open = false
				getter = nil // don't take anymore requests
			case now := <-ticker.C:
				if err := ping(now); err != nil {
					logger.Println("ping error:", err)
					break loop
				}
			case r, ok := <-getter:
				logger.Println("getter got")
				if !ok {
					logger.Println("getter is closed")
					break loop
				}
				logger.Println("getter sending stuff")
				if err := send(r); err != nil {
					logger.Println("getter error sending stuff:", err)
					teller <- Results{ErrMsg: err.Error()}
					continue
				}
				logger.Println("getter sent stuff")
				active = true
			case r, ok := <-response:
				if !ok {
					break loop
				}
				var results Results
				if err := json.NewDecoder(r).Decode(&results); err != nil {
					logger.Println("status json error:", err)
					results = Results{ErrMsg: err.Error()}
				}
				teller <- results
				active = false
				logger.Println("read complete")
			}
		}
		logger.Println("websocket server closing")
		msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		if err := conn.WriteMessage(websocket.CloseMessage, msg); err != nil {
			logger.Println("websocket server close message error:", err)
		}
		conn.Close()
		close(teller)
		logger.Println("exit pusher ")
	}
}
