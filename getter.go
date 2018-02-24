// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// Actionable functions process an io.Reader and returns
// any relevant results
// a bool set false if to close the client,
// and an error if such is encountered
type Actionable func(io.Reader) (interface{}, bool, error)

// Client connects to url and applies the Actionable function to each message received
// url specifices the websocket endpoint to connect to
// pings will log websocket pings if set true
// headers supplies optional http headers for authentication
// logger logs actions
func Client(url string, fn Actionable, pings bool, headers http.Header, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(os.Stderr, "client ", LogFlags)
	}
	conn, err := dial(url, headers, logger)
	if err != nil {
		return err
	}

	if pings {
		pingHandler := conn.PingHandler()
		conn.SetPingHandler(func(s string) error {
			logger.Print("GOT A PING:", s)
			return pingHandler(s)
		})
	}

	return client(conn, fn, logger)
}

// client applies the Actionable function to the websocket connection
func client(conn *websocket.Conn, fn Actionable, logger *log.Logger) error {

	defer func() {
		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		logger.Println("cleaning up and closing")
		msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		err := conn.WriteMessage(websocket.CloseMessage, msg)
		if err != nil && websocket.IsUnexpectedCloseError(err, 1000) {
			logger.Println("websocket CloseMessage error:", err)
		}
		conn.Close()
	}()

	var err error

	for ok := true; ok; {
		logger.Println("client waiting for message")
		messageType, r, err := conn.NextReader()
		if err != nil {
			if err != nil && websocket.IsCloseError(err, 1000) {
				return nil
			}
			logger.Println("NextReader error:", err)
			return err
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

		if messageType != websocket.BinaryMessage {
			logger.Printf("UNKNOWN MSG (%T):", messageType)
			io.Copy(os.Stdout, r)
			continue
		}

		var reply interface{}
		reply, ok, err = fn(r)
		if err != nil {
			logger.Println("client function ok:", ok, "err:", err)
		}

		var results Results
		if err != nil {
			results.ErrMsg = err.Error()
		}

		if reply != nil {
			b, err := json.Marshal(reply)
			if err != nil {
				logger.Println("reply json error:", err)
				continue
			}
			raw := json.RawMessage(b)
			results.Payload = &raw
		}

		if err := conn.WriteJSON(results); err != nil {
			logger.Println("results status json error:", err)
			return errors.Wrap(err, "status write error")
		}

	}
	logger.Println("client returning error:", err)
	return err
}

// dial connects to url and return a websocket connection if successful
func dial(url string, headers http.Header, logger *log.Logger) (*websocket.Conn, error) {
	if logger == nil {
		logger = log.New(os.Stderr, "client ", LogFlags)
	}
	if strings.HasPrefix(url, "http") {
		url = "ws" + url[4:]
	}
	logger.Println("connecting to:", url)
	conn, resp, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		if resp != nil {
			if resp.Body != nil {
				io.Copy(os.Stderr, resp.Body)
			}
			return nil, errors.Wrapf(err, "dial code:%d status:%s", resp.StatusCode, resp.Status)
		}
		return nil, errors.Wrap(err, "websocket dial error for url: "+url)
	}

	logger.Println("connected")

	return conn, nil
}
