// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paulstuart/websox"
)

var (
	port         = os.Getenv("PORT")
	addr         = flag.String("addr", ":"+port, "http service address")
	ssl          = flag.Bool("ssl", false, "ssl terminated")
	upgrader     = websocket.Upgrader{} // use default options
	homeTemplate *template.Template
	wsProto      = "ws"

	//homeTemplate = template.Must(template.New("").ParseFiles("index.html"))
)

func init() {
	var err error
	homeTemplate, err = template.ParseFiles("index.html")
	if err != nil {
		panic(err)
	}
}

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func push(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("push upgrade error:", err)
		return
	}
	defer c.Close()
	for {
		stuff := websox.FakeStuff()
		if b, err := json.Marshal(stuff); err != nil {
			log.Println("json error:", err)
			continue
		} else {
			if err = c.WriteMessage(websocket.TextMessage, b); err != nil {
				log.Println("push write error:", err)
				break
			}
		}

		time.Sleep(time.Second * 1)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	if err := homeTemplate.Execute(w, wsProto+"://"+r.Host+"/echo"); err != nil {
		fmt.Println("home error:", err)
	}
}

func pusher(w http.ResponseWriter, r *http.Request) {
	if err := homeTemplate.Execute(w, wsProto+"://"+r.Host+"/echo"); err != nil {
		fmt.Println("home error:", err)
	}
}

func main() {
	flag.Parse()
	if *ssl {
		wsProto += "s"
	}
	log.SetFlags(0)

	http.HandleFunc("/pusher", pusher)
	http.HandleFunc("/push", push)
	http.HandleFunc("/echo", echo)
	http.HandleFunc("/", home)
	http.HandleFunc("/ok", home)
	fmt.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
