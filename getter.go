// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"compress/zlib"
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

const logFlags = log.Ldate | log.Lmicroseconds | log.Lshortfile

// Client will connect to url and apply the Actionable function to each message recieved
func Client(url string, fn Actionable, pings bool, headers http.Header) error {
	logger := log.New(os.Stderr, "client ", logFlags)
	if strings.HasPrefix(url, "http") {
		url = "ws" + url[4:]
	}
	logger.Printf("connecting to %s", url)
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
		logger.Printf("got close code: %d text: %s\n", code, text)
		if closeHandler != nil {
			logger.Println("calling original closeHandler")
			err := closeHandler(code, text)
			if err != nil && websocket.IsUnexpectedCloseError(err, 1000) {
				logger.Println(" closeHandler error:", err)
				return err
			}
			return nil
		}
		return nil
	})

	defer func() {
		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		logger.Println("cleaning up and closing")
		err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil && websocket.IsUnexpectedCloseError(err, 1000) {
			logger.Println("websocket close error:", err)
		}
		conn.Close()
	}()

	if pings {
		pingHandler := conn.PingHandler()
		conn.SetPingHandler(func(s string) error {
			logger.Print("GOT A PING:", s)
			return pingHandler(s)
		})
	}

	logger.Printf("connected to %s", url)
	ok := true
	for ok {
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

		// decompress the message before the function sees it
		var reply interface{}
		z, err := zlib.NewReader(r)
		if err == nil {
			reply, ok, err = fn(z)
			if err != nil {
				logger.Println("ok:", ok, "fn err:", err)
			}
		}
		z.Close()

		var errMsg string
		if err != nil {
			errMsg = err.Error()
		}
		results := Results{
			ErrMsg:  errMsg,
			Payload: reply,
		}

		b, jerr := json.Marshal(results)
		if jerr != nil {
			log.Println("status json error:", jerr)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return errors.Wrap(err, "status write error")
		}
		logger.Println("replied results:", results)
	}
	log.Println("client returning error:", err)
	return err
}
