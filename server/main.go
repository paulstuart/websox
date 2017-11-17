// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.build.ge.com/Aviation-APM/oauth2-auth"
	"github.com/paulstuart/websox"
)

var (
	addr  *string
	delay *bool
)

func init() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}
	addr = flag.String("addr", ":"+port, "http service address")
	delay = flag.Bool("delay", false, "add randomized delay")
}

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello")
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	valid, err := oauth2auth.MakeValidatorFromEnvironment()
	if err != nil {
		panic(err)
	}

	var expires *time.Duration
	if refresh_period := os.Getenv("refresh_period"); len(refresh_period) > 0 {
		dur, err := time.ParseDuration(refresh_period)
		if err != nil {
			panic(err)
		}
		expires = &dur
	}

	fmt.Println("setting up http handlers")
	http.HandleFunc("/push", valid.AuthorizationRequired(websox.Pusher(fakeLoop, expires)))
	http.HandleFunc("/", home)

	fmt.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
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

			if *delay {
				pause := rand.Intn(5)
				time.Sleep(time.Second * time.Duration(pause))
			}
		}
	}()

	return getter, teller
}
