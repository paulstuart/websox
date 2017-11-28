// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paulstuart/websox"
)

var (
	uaa_client_id     = os.Getenv("uaa_client_id")
	uaa_client_secret = os.Getenv("uaa_client_secret")
	uaa_url           = os.Getenv("uaa_url")
	server_addr       = os.Getenv("server_addr")
)

func main() {

	if len(server_addr) == 0 {
		server_addr = "localhost:8080"
	}
	addr := flag.String("addr", server_addr, "http service address")
	debug := flag.Bool("debug", false, "enable debugging")
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/push"}
	if strings.HasSuffix(*addr, "443") {
		u.Scheme += "s"
	}

	if *debug {
		fmt.Println("connecting to:", u.String())
		fmt.Printf("CID: %s SECRET: %s URL: %s\n", uaa_client_id, uaa_client_secret, uaa_url)
	}

	ctx := context.Background()
	for {
		headers, err := websox.Oauth2Header(ctx, uaa_url, uaa_client_id, uaa_client_secret)
		if err != nil {
			panic(err)
		}
		err = websox.Client(u.String(), gotIt, headers)
		if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			fmt.Printf("(%T) %v\n", err, err)
		}
		fmt.Println("sleep for a minute")
		time.Sleep(time.Minute)
	}
}

func gotIt(r io.Reader) (bool, error) {
	var s websox.Stuff
	ok := true
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		fmt.Println("json error:", err)
		return ok, err
	}
	fmt.Println("Stuff count:", s.Count)
	three := s.Count%3 == 0
	five := s.Count%5 == 0
	switch {
	case three && five:
		return ok, fmt.Errorf("fizzbuzz")
	case three:
		return ok, fmt.Errorf("fizz")
	case five:
		return ok, fmt.Errorf("buzz")
	}
	return ok, nil
}
