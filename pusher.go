// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"compress/zlib"
	"encoding/json"
	"fmt"
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
// The error channel is closed by Pusher() when it id done processing (due to timeout or error)
type Setup func() (chan interface{}, chan Results)

// Pusher gets send/recv channels from the setup function
// and apply the channel data to a websocket connection
func Pusher(setup Setup, expires, pingFreq time.Duration) http.HandlerFunc {
	const flags = log.Ldate | log.Lmicroseconds | log.Lshortfile
	return func(w http.ResponseWriter, r *http.Request) {
		logger := log.New(os.Stderr, pusherID(), flags)

		// TODO: should we do origin checking?
		//fmt.Printf("pusher origin:%s host:%s\n", r.Header.Get("Origin"), r.Host)

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
			log.Print("push upgrade error:", err)
			return
		}

		sending, reading := true, true

		closeHandler := conn.CloseHandler()
		conn.SetCloseHandler(func(code int, text string) error {
			sending = false
			log.Printf("got close code: %d text: %s\n", code, text)
			if closeHandler != nil {
				logger.Println("calling original closeHandler")
				return closeHandler(code, text)
			}
			return nil
		})

		conn.SetPongHandler(func(s string) error {
			log.Print("SERVER GOT A PONG:", s)
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

		send := func(msg interface{}) error {
			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return err
			}

			// compress data before sending
			z := zlib.NewWriter(w)
			if err = json.NewEncoder(z).Encode(msg); err != nil {
				z.Close()
				w.Close()
				return err
			}

			if err := z.Close(); err != nil {
				w.Close()
				return err
			}

			return w.Close()
		}

		timeout := make(<-chan time.Time)
		if expires != 0 {
			timeout = time.NewTimer(expires).C
		}

		quit := make(chan struct{})
		go func() {
			for reading {
				logger.Println("waiting for reply")
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					logger.Println("pusher read error:", err)
					break
				}
				logger.Println("we have a reply")

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
					teller <- Results{ErrMsg: err.Error()}
					continue
				}

				//go func() {
				logger.Println("telling teller")
				teller <- results
				logger.Println("told teller")
				//}()
			}
			logger.Println("====> read loop complete")
			quit <- struct{}{}
			logger.Println("====> exiting read loop")
		}()

	loop:
		for sending {
			select {
			case <-timeout:
				logger.Println("session has expired")
				break loop
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
				logger.Println("sending stuff")
				if err := send(stuff); err != nil {
					logger.Println("error sending stuff:", err)
					teller <- Results{ErrMsg: err.Error()}
				}
			}
		}
		reading = false
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
