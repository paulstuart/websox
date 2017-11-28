// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"compress/zlib"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = time.Second
)

// Setup returns channels to get data and return the error when trying to save said data
// If the setup function cannot do the required processing,
// it should return a nil interface channel and send an error message in the error channel
//
// The error channel is closed by Pusher() when it id done processing (due to timeout or error)
type Setup func() (chan interface{}, chan error)

// Pusher gets send/recv channels from the setup function
// and apply the channel data to a websocket connection
func Pusher(setup Setup, expires, pingFreq time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// TODO: should we do origin checking?
		//fmt.Printf("pusher origin:%s host:%s\n", r.Header.Get("Origin"), r.Host)

		getter, teller := setup()
		if getter == nil {
			err := <-teller
			close(teller)
			log.Println("Pusher setup error:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		upgrader := websocket.Upgrader{} // use default options
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("push upgrade error:", err)
			return
		}

		closeHandler := conn.CloseHandler()
		conn.SetCloseHandler(func(code int, text string) error {
			log.Printf("got close code: %d text: %s\n", code, text)
			if closeHandler != nil {
				log.Println("calling original closeHandler")
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

		go func() {
			for {
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					log.Println("pusher read error:", err)
					break
				}

				switch messageType {
				case websocket.CloseMessage:
					log.Println("uhoh! closing time!")
					break
				case websocket.PingMessage:
					log.Println("PING!")
					continue
				case websocket.PongMessage:
					log.Println("PONG!")
					continue
				}

				var status ErrMsg
				if err := json.Unmarshal(message, &status); err != nil {
					log.Println("status json error:", err)
					teller <- err
					continue
				}

				teller <- status.Error()
			}
			log.Println("existing read loop")
		}()

		for {
			select {
			case <-timeout:
				log.Println("session has expired")
				goto DONE
			case now := <-ticker.C:
				log.Println("time to ping:", now, "err:", ping(now))
			case stuff, ok := <-getter:
				if !ok {
					log.Println("getter is closed")
					goto DONE
				}
				if err := send(stuff); err != nil {
					teller <- err
				}
			}
		}

	DONE:
		log.Println("websocket server closing")
		close(teller)
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("websocket server close error:", err)
		}
		conn.Close()
	}
}
