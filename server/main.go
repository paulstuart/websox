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
	_ "net/http/pprof"
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

	log.Println("setting up http handlers")
	log.Println(
		"uaa_client_id:", os.Getenv("uaa_client_id"),
		"uaa_client_secret:", os.Getenv("uaa_client_secret"),
		"uaa_url:", os.Getenv("uaa_url"),
	)
	http.HandleFunc("/push", valid.AuthorizationRequired(websox.Pusher(fakeLoop, expires)))
	http.HandleFunc("/", home)

	log.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func fakeLoop() (chan interface{}, chan error) {
	var i int

	getter := make(chan interface{})
	teller := make(chan error)

	go func() {
		for {
			i++
			if i%1000 == 0 {
				log.Println("waiting for a couple minutes")
				time.Sleep(time.Minute * 2)
			}

			getter <- websox.Stuff{
				Msg:   fmt.Sprintf("msg number: %d", i),
				Count: i,
				TS:    time.Now(),
			}
			err, ok := <-teller
			if !ok {
				fmt.Println("teller must be closed")
				break
			}
			if err != nil {
				fmt.Println("fakeloop got error:", err)
			}

			if *delay {
				pause := rand.Intn(5)
				time.Sleep(time.Second * time.Duration(pause))
			}
		}
		log.Println("fakeLoop is closing")
		close(getter)
	}()

	return getter, teller
}
