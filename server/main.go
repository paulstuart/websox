// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

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

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello")
}

func main() {
	flag.Parse()
	if *ssl {
		wsProto += "s"
	}
	log.SetFlags(0)

	http.HandleFunc("/push", websox.Pusher(fakeLoop))
	http.HandleFunc("/", home)

	fmt.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func fakeloop() (chan interface{}, chan error) {
	var i int

	getter := make(chan interface{})
	teller := make(chan error)

	go func() {
		for {
			i++
			getter <- websox.stuff{
				msg:   fmt.sprintf("msg number: %d", i),
				count: i,
				ts:    time.now(),
			}
			err := <-teller
			if err != nil {
				fmt.println("fakeloop got error:", err)
			}
			delay := rand.intn(5)
			time.sleep(time.second * time.duration(delay))
		}
	}()

	return getter, teller
}
