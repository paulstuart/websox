// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/paulstuart/websox"
)

var (
	addr = flag.String("addr", "localhost:8080", "http service address")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/push"}
	if strings.HasSuffix(*addr, "443") {
		u.Scheme += "s"
	}
	Client(u.String(), gotIt)
}

// Actionable functions act on a received websocket message and return an error if they failed
type Actionable func([]byte) error

// Client will connect to url and apply the Actionable function to each message recieved
func Client(url string, fn Actionable) {
	log.Printf("connecting to %s", url)

	requestHeader := http.Header{}
	requestHeader.Add("Origin", url)

	c, _, err := websocket.DefaultDialer.Dial(url, requestHeader)
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
		var status websox.ErrMsg
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

var cnt int

func gotIt(message []byte) error {
	cnt++
	fmt.Println("recv cnt:", cnt)
	var s websox.Stuff
	if err := json.Unmarshal(message, &s); err != nil {
		return err
	}
	three := s.Count%3 == 0
	five := s.Count%5 == 0
	switch {
	case three && five:
		return fmt.Errorf("fizzbuzz")
	case three:
		return fmt.Errorf("fizz")
	case five:
		return fmt.Errorf("fizzbuzz")
	}
	return nil
}
