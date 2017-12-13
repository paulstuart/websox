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
	"sync"
	"time"

	"github.build.ge.com/Aviation-APM/oauth2-auth"
	"github.com/paulstuart/websox"
)

var (
	addr   *string
	delay  *bool
	mu     sync.Mutex
	wg     sync.WaitGroup
	locked bool
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
	fmt.Fprintln(w, "hello, whirled")
}

func lock(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	if locked {
		wg.Done()
	} else {
		wg.Add(1)
	}
	locked = !locked
	fmt.Fprintln(w, "locked:", locked)
	mu.Unlock()
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	period := os.Getenv("ping_period")
	pingPeriod, err := time.ParseDuration(period)
	if len(period) > 0 && err != nil {
		log.Fatal(err)
	}

	valid, err := oauth2auth.MakeValidatorFromEnvironment()
	if err != nil {
		panic(err)
	}

	var expires time.Duration
	if refresh_period := os.Getenv("refresh_period"); len(refresh_period) > 0 {
		expires, err = time.ParseDuration(refresh_period)
		if err != nil {
			panic(err)
		}
	}

	log.Println("setting up http handlers")
	log.Println(
		"uaa_client_id:", os.Getenv("uaa_client_id"),
		"uaa_client_secret:", os.Getenv("uaa_client_secret"),
		"uaa_url:", os.Getenv("uaa_url"),
	)
	http.HandleFunc("/push", valid.AuthorizationRequired(websox.Pusher(fakeLoop, expires, pingPeriod)))
	http.HandleFunc("/lock", lock)
	http.HandleFunc("/", home)

	log.Println("listening on:", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func fakeLoop() (chan interface{}, chan websox.Results) {
	var i int

	getter := make(chan interface{})
	teller := make(chan websox.Results)

	go func() {
		for {
			// turn off and on by accessing /lock on the server side
			wg.Wait()
			i++

			getter <- websox.Stuff{
				Msg:   fmt.Sprintf("msg number: %d", i),
				Count: i,
				TS:    time.Now(),
			}
			results, ok := <-teller
			if !ok {
				fmt.Println("teller must be closed")
				break
			}
			if false && results.Err != nil {
				fmt.Println("fakeloop got error:", results.Err)
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
