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
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	writeWait = time.Second
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

		var stop time.Time
		if expires != nil {
			stop = time.Now().Add(*expires)
		}

		for {
			if expires != nil && time.Now().After(stop) {
				log.Println("timeout for duration:", *expires)
				break
			}

			log.Println("waiting for getter")
			stuff, ok := <-getter
			if !ok {
				log.Println("getter is closed")
				break
			}
			log.Println("marshaling stuff:", stuff)
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

			if err = conn.WriteMessage(websocket.BinaryMessage, buff.Bytes()); err != nil {
				log.Println("pusher write error:", err)
				teller <- err
				break
			}

			_, message, err := conn.ReadMessage()
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
		log.Println("websocket server closing")
		close(teller)
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("websocket server close error:", err)
		}
		conn.Close()
	}
}

// Actionable functions process an io.Reader and returns
// a bool set false if to close the client, and an error if such is encountered
type Actionable func(io.Reader) (bool, error)

// Client will connect to url and apply the Actionable function to each message recieved
func Client(url string, fn Actionable, pingFreq time.Duration, headers http.Header) error {
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

	ping := func(now time.Time) error {
		err := conn.WriteControl(websocket.PingMessage, []byte(now.String()), now.Add(writeWait))
		if err == websocket.ErrCloseSent {
			return nil
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		}
		return err
	}

	notify := make(chan os.Signal, 1)
	signal.Notify(notify, os.Interrupt)
	ticker := time.NewTicker(pingFreq)

	cleanup := func() {
		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		log.Println("cleaning up and closing")
		err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("websocket close error:", err)
		}
		conn.Close()
		ticker.Stop()
	}

	defer cleanup()

	go func() {
		what := <-notify
		log.Println("interrupt:", what)

		cleanup()
		os.Exit(0)
	}()

	go func() {
		log.Println("starting ticker:", ticker)
		for now := range ticker.C {
			fmt.Println("PING:", now)

			if err := ping(now); err != nil {
				log.Println("error sending ping:", err)
			}
			/*
				if err := conn.WriteControl(websocket.PingMessage, []byte(now.String())); err != nil {
					log.Println("error sending ping:", err)
				}
			*/
		}
	}()

	conn.SetPongHandler(func(s string) error {
		log.Print("GOT A PONG:", s)
		return nil
	})

	log.Printf("connected to %s", url)
	ok := true
	for ok {
		messageType, r, err := conn.NextReader()
		if err != nil {
			log.Println("NextReader error:", err)
			return err
		}

		if messageType != websocket.BinaryMessage {
			log.Printf("MSG (%T):", messageType)
			io.Copy(os.Stdout, r)
		}

		if messageType == websocket.CloseMessage {
			log.Println("got websocket close message")
			return nil
		}

		// decompress the message before the function sees it
		z, err := zlib.NewReader(r)
		if err == nil {
			ok, err = fn(z)
			log.Println("fn err:", err)
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

		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return errors.Wrap(err, "status write error")
		}
	}
	return nil
}
