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
type Setup func() (chan io.ReadCloser, chan Results)

// Pusher gets send/recv channels from the setup function
// and apply the channel data to a websocket connection
func Pusher(setup Setup, expires, pingFreq time.Duration, contacted func(), logger *log.Logger) http.HandlerFunc {
	const logFlags = log.Ldate | log.Lmicroseconds | log.Lshortfile
	return func(w http.ResponseWriter, r *http.Request) {
		if logger == nil {
			logger = log.New(os.Stderr, pusherID(), logFlags)
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

		quit := make(chan struct{})
		complete := make(chan struct{})

		closeHandler := conn.CloseHandler()
		conn.SetCloseHandler(func(code int, text string) error {
			quit <- struct{}{}
			logger.Printf("got close code: %d text: %s\n", code, text)
			if closeHandler != nil {
				logger.Println("calling original closeHandler")
				return closeHandler(code, text)
			}
			return nil
		})

		conn.SetPongHandler(func(s string) error {
			if contacted != nil {
				contacted()
			}
			return nil
		})

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

		go func() {
			for {
				logger.Println("waiting for reply")
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					logger.Println("pusher read error:", err)
					break
				}
				logger.Println("we have a reply")
				if contacted != nil {
					contacted()
				}

				switch messageType {
				case websocket.CloseMessage:
					logger.Println("uhoh! closing time!")
					break
				case websocket.PingMessage:
					logger.Println("PING!")
					continue
				case websocket.PongMessage:
					logger.Println("PONG!")
					continue
				}

				var results Results
				if err := json.Unmarshal(message, &results); err != nil {
					logger.Println("status json data:", string(message), "error:", err)
					results = Results{ErrMsg: err.Error()}
				}

				logger.Println("telling teller")
				teller <- results
				logger.Println("told teller")
				complete <- struct{}{}
			}
			logger.Println("====> read loop complete")
			quit <- struct{}{}
			logger.Println("====> exiting read loop")
		}()

		expired := make(<-chan time.Time)
		if expires != 0 {
			expired = time.NewTimer(expires).C
		}

		active, open := false, true

	loop:
		for open || active {
			//logger.Println("ACTIVE:", active, "OPEN:", open)
			select {
			case <-complete:
				logger.Println("read complete")
				active = false
			case <-expired:
				logger.Println("session has expired")
				open = false
				getter = nil // don't take anymore requests
			case <-quit:
				logger.Println("it's quitting time")
				break loop
			case now := <-ticker.C:
				if err := ping(now); err != nil {
					logger.Println("ping error:", err)
					break loop
				}
			case stuff, ok := <-getter:
				logger.Println("getter got stuff")
				if !ok {
					logger.Println("getter is closed")
					break loop
				}
				logger.Println("getter sending stuff")
				if err := send(stuff); err != nil {
					logger.Println("getter error sending stuff:", err)
					teller <- Results{ErrMsg: err.Error()}
					continue
				}
				active = true
			}
		}
		logger.Println("websocket server closing")
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			logger.Println("websocket server close error:", err)
		}
		conn.Close()
		<-quit
		logger.Println("close teller")
		close(teller)
		logger.Println("pusher done")
	}
}
