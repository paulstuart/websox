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
	"time"

	"github.com/gorilla/websocket"
	"github.com/paulstuart/websox"
)

var (
	addr = flag.String("addr", "localhost:8080", "http service address")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	//u := url.URL{Scheme: "ws", Host: *addr, Path: "/echo"}
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/push"}
	if strings.HasSuffix(*addr, "443") {
		u.Scheme += "s"
	}
	log.Printf("connecting to %s", u.String())

	/*
		dialer := websocket.Dialer{
			 Proxy: http.ProxyFromEnvironment,

		}
	*/
	requestHeader := http.Header{}
	requestHeader.Add("Origin", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), requestHeader)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer c.Close()
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			var s websox.Stuff
			if err := json.Unmarshal(message, &s); err != nil {
				log.Println("json error:", err)
				continue
			}
			//log.Printf("recv: %s", message)
			log.Printf("recv: %-v", s)
			three := s.Count%3 == 0
			five := s.Count%5 == 0
			//fmt.Printf("CNT:%d 3:%d 5:%d\n", s.Count, s.Count%3, s.Count%5)
			var msg string
			switch {
			case three && five:
				msg = "fizzbuzz"
			case three:
				msg = "fizz"
			case five:
				msg = "fizzbuzz"
			}
			status := websox.Status{
				Msg: msg,
				Ok:  len(msg) == 0,
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
	}()

	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()

	for {
		select {
		case t := <-ticker.C:
			log.Println("TICK:", t)
			/*
				msg := []byte("it has been " + t.String())
				//if err := c.WriteMessage(websocket.TextMessage, []byte(t.String())); err != nil {
				if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Println("write error:", err)
					return
				}
			*/
		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			c.Close()
			return
		}
	}
}
