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
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// Actionable functions process an io.Reader and returns
// any relevant results
// a bool set false if to close the client,
// and an error if such is encountered
type Actionable func(io.Reader) (interface{}, bool, error)

// Client will connect to url and apply the Actionable function to each message recieved
func Client(url string, fn Actionable, pings bool, headers http.Header) error {
	log.Printf("connecting to %s", url)

	conn, resp, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		if resp != nil {
			if resp.Body != nil {
				io.Copy(os.Stdout, resp.Body)
			}
			return errors.Wrapf(err, "dial code:%d status:%s", resp.StatusCode, resp.Status)
		}
		return errors.Wrap(err, "websocket dial error for url: "+url)
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

	defer func() {
		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		log.Println("cleaning up and closing")
		err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("websocket close error:", err)
		}
		conn.Close()
	}()

	if pings {
		pingHandler := conn.PingHandler()
		conn.SetPingHandler(func(s string) error {
			log.Print("GOT A PING:", s)
			return pingHandler(s)
		})
	}

	log.Printf("connected to %s", url)
	ok := true
	for ok {
		messageType, r, err := conn.NextReader()
		if err != nil {
			log.Println("NextReader error:", err)
			return err
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

		if messageType != websocket.BinaryMessage {
			log.Printf("UNKNOWN MSG (%T):", messageType)
			io.Copy(os.Stdout, r)
			continue
		}

		// decompress the message before the function sees it
		var reply interface{}
		z, err := zlib.NewReader(r)
		if err == nil {
			reply, ok, err = fn(z)
			if err != nil {
				log.Println("fn err:", err)
			}
		}
		z.Close()

		results := Results{
			Err:     err,
			Payload: reply,
		}

		b, err := json.Marshal(results)
		if err != nil {
			fmt.Println("status json error:", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return errors.Wrap(err, "status write error")
		}
	}
	return nil
}
