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
	addr         *string
	ssl          = flag.Bool("ssl", false, "ssl terminated")
	upgrader     = websocket.Upgrader{} // use default options
	homeTemplate *template.Template
	wsProto      = "ws"

	//homeTemplate = template.Must(template.New("").ParseFiles("index.html"))
)

func init() {
	if len(port) == 0 {
		port = "8080"
	}
	addr = flag.String("addr", ":"+port, "http service address")
	var err error
	homeTemplate, err = template.ParseFiles("index.html")
	if err != nil {
		panic(err)
	}
}

func echo(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("echo origin:%s host:%s\n", r.Header.Get("Origin"), r.Host)
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
		var status websox.Status
		fmt.Println("STATUS:", string(message))
		if err := json.Unmarshal(message, &status); err != nil {
			log.Println("status json error:", err)
			continue
		}

		if !status.Ok {
			fmt.Println("crap:", status.Msg)
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
