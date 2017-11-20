// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
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

// Setup returns channels to get data and return the error when trying to save said data
// If the setup function cannot do the required processing,
// it should return a nil interface channel and send an error message in the error channel
//
// The error channel is closed by Pusher() when it id done processing (due to timeout or error)
type Setup func() (chan interface{}, chan error)

// Pusher gets send/recv channels from the setup function
// and apply the channel data to a websocket connection
func Pusher(setup Setup, expires *time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// TODO: should we do origin checking?
		//fmt.Printf("pusher origin:%s host:%s\n", r.Header.Get("Origin"), r.Host)

		getter, teller := setup()
		if getter == nil {
			err := <-teller
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		upgrader := websocket.Upgrader{} // use default options
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("push upgrade error:", err)
			return
		}

		var stop time.Time
		if expires != nil {
			stop = time.Now().Add(*expires)
		}

		for {
			if expires != nil && time.Now().After(stop) {
				log.Println("timeout for duration:", *expires)
				break
			}

			stuff, ok := <-getter
			if !ok {
				break
			}
			b, err := json.Marshal(stuff)
			if err != nil {
				log.Println("pusher json error:", err)
				teller <- err
				continue
			}

			// compress data before sending
			var buff bytes.Buffer
			z := zlib.NewWriter(&buff)
			z.Write(b)
			z.Close()

			if err = c.WriteMessage(websocket.BinaryMessage, buff.Bytes()); err != nil {
				log.Println("pusher write error:", err)
				teller <- err
				break
			}

			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("pusher read error:", err)
				teller <- err
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
		err = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("websocket server close error:", err)
		}
		c.Close()
	}
}

// Actionable functions process an io.Reader and return an error if such is encountered
type Actionable func(io.Reader) error

// Client will connect to url and apply the Actionable function to each message recieved
func Client(url string, fn Actionable, headers http.Header) error {
	log.Printf("connecting to %s", url)

	c, resp, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		io.Copy(os.Stdout, resp.Body)
		return errors.Wrapf(err, "dial code:%d status:%s", resp.StatusCode, resp.Status)
	}
	defer c.Close()

	notify := make(chan os.Signal, 1)
	signal.Notify(notify, os.Interrupt)

	go func() {
		what := <-notify
		log.Println("interrupt:", what)

		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("websocket close error:", err)
		}
		c.Close()
		os.Exit(0)
	}()

	for {
		messageType, r, err := c.NextReader()
		if err != nil {
			return err
		}

		if messageType != websocket.BinaryMessage {
			fmt.Printf("MSG (%T):", messageType)
			io.Copy(os.Stdout, r)
		}
		if messageType == websocket.CloseMessage {
			log.Println("got websocket close message")
			return nil
		}

		// decompress the message before the function sees it
		z, err := zlib.NewReader(r)
		if err == nil {
			err = fn(z)
		}

		var status ErrMsg
		if err != nil {
			status.Msg = err.Error()
		}
		z.Close()

		b, err := json.Marshal(status)
		if err != nil {
			fmt.Println("status json error:", err)
			continue
		}

		if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
			return errors.Wrap(err, "status write error")
		}
	}
	return nil
}
