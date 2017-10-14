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
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paulstuart/websox"
)

var (
	port     = os.Getenv("PORT")
	addr     *string
	ssl      = flag.Bool("ssl", false, "ssl terminated")
	upgrader = websocket.Upgrader{} // use default options
	wsProto  = "ws"
)

func init() {
	if len(port) == 0 {
		port = "8080"
	}
	addr = flag.String("addr", ":"+port, "http service address")
}

func push(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("push origin:%s host:%s\n", r.Header.Get("Origin"), r.Host)
	/*
		if r.Header.Get("Origin") != "http://"+r.Host {
			http.Error(w, "Origin not allowed", 403)
			return
		}
	*/
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("push upgrade error:", err)
		return
	}
	defer c.Close()
	for {
		stuff := websox.FakeStuff()
		b, err := json.Marshal(stuff)
		if err != nil {
			log.Println("stuff json error:", err)
			continue
		}

		if err = c.WriteMessage(websocket.TextMessage, b); err != nil {
			log.Println("push write error:", err)
			break
		}

		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("push read error:", err)
			break
		}
		var status websox.ErrMsg
		fmt.Println("STATUS:", string(message))
		if err := json.Unmarshal(message, &status); err != nil {
			log.Println("status json error:", err)
			continue
		}
		if err = status.Error(); err != nil {
			fmt.Println("crap:", err)
		}

		time.Sleep(time.Second * 1)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello")
}

func main() {
	flag.Parse()
	if *ssl {
		wsProto += "s"
	}
	log.SetFlags(0)

	http.HandleFunc("/push", push)
	http.HandleFunc("/", home)

	fmt.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
