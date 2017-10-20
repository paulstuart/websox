// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

// ErrMsg is for reporting errors on websocket pushes
type ErrMsg struct {
	Msg string `json:"errmsg"`
}

// Error returns an error if any error message is returned
func (e ErrMsg) Error() error {
	if len(e.Msg) == 0 {
		return nil
	}
	return fmt.Errorf(e.Msg)
}

// Admin returns channels to get data and return the error when trying to save said data
type Admin func() (chan interface{}, chan error)

type Validator func(*http.Request) error

// Pusher will apply Admin functionality to a websocket server connection
func Pusher(admin Admin, valid Validator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if valid != nil {
			if err := valid(r); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
		}
		fmt.Printf("pusher origin:%s host:%s\n", r.Header.Get("Origin"), r.Host)
		getter, teller := admin()
		upgrader := websocket.Upgrader{} // use default options
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("push upgrade error:", err)
			return
		}
		defer c.Close()
		for {
			stuff, ok := <-getter
			if !ok {
				break
			}
			b, err := json.Marshal(stuff)
			if err != nil {
				log.Println("gusher json error:", err)
				continue
			}

			if err = c.WriteMessage(websocket.TextMessage, b); err != nil {
				log.Println("gusher write error:", err)
				break
			}

			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("gusher read error:", err)
				break
			}

			var status ErrMsg
			if err := json.Unmarshal(message, &status); err != nil {
				log.Println("status json error:", err)
				teller <- err
				continue
			}

			teller <- status.Error()
		}
	}
}

// Actionable functions do something with a byte slice and return an error if such is encountered
type Actionable func([]byte) error

// Client will connect to url and apply the Actionable function to each message recieved
func Client(url string, fn Actionable, headers http.Header) {
	log.Printf("connecting to %s", url)

	c, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		what := <-interrupt
		log.Println("interrupt:", what)

		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("websocket close error:", err)
		}
	}()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("websocket read error:", err)
			return
		}

		var status ErrMsg
		if err = fn(message); err != nil {
			status.Msg = err.Error()
		}

		b, err := json.Marshal(status)
		if err != nil {
			fmt.Println("status json error:", err)
			continue
		}

		if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
			log.Println("status write error:", err)
			return
		}
	}
}
