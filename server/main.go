// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/paulstuart/websox"
)

var (
	port = os.Getenv("PORT")
	addr *string
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
	log.SetFlags(0)

	fmt.Println("setting up http handlers")
	http.HandleFunc("/push", websox.Pusher(fakeLoop, notVerySafe))
	http.HandleFunc("/", home)

	fmt.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func notVerySafe(r *http.Request) error {
	secret := r.Header.Get("x-secret-code")
	if len(secret) == 0 {
		return fmt.Errorf("no secret given")
	}
	if secret != "Open Sesame!" {
		return fmt.Errorf("bad secret")
	}
	return nil
}

func fakeLoop() (chan interface{}, chan error) {
	var i int

	getter := make(chan interface{})
	teller := make(chan error)

	go func() {
		for {
			i++
			getter <- websox.Stuff{
				Msg:   fmt.Sprintf("msg number: %d", i),
				Count: i,
				TS:    time.Now(),
			}
			err := <-teller
			if err != nil {
				fmt.Println("fakeloop got error:", err)
			}
			delay := rand.Intn(5)
			time.Sleep(time.Second * time.Duration(delay))
		}
	}()

	return getter, teller
}
