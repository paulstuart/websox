// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
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

func home(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/echo", echo)
	http.HandleFunc("/", home)
	http.HandleFunc("/ok", home)
	fmt.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
