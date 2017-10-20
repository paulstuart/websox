// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/paulstuart/websox"
)

var (
	addr = flag.String("addr", "localhost:8080", "http service address")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/push"}
	if strings.HasSuffix(*addr, "443") {
		u.Scheme += "s"
	}
	headers := make(http.Header)
	headers.Add("x-secret-code", "Open Sesame!")
	websox.Client(u.String(), gotIt, headers)
}

var cnt int

func gotIt(message []byte) error {
	cnt++
	fmt.Println("recv cnt:", cnt)
	var s websox.Stuff
	if err := json.Unmarshal(message, &s); err != nil {
		return err
	}
	three := s.Count%3 == 0
	five := s.Count%5 == 0
	switch {
	case three && five:
		return fmt.Errorf("fizzbuzz")
	case three:
		return fmt.Errorf("fizz")
	case five:
		return fmt.Errorf("buzz")
	}
	return nil
}
