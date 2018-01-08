// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Results is used to return client results / errors on websocket pushes
type Results struct {
	ErrMsg  string           `json:"error"`
	Payload *json.RawMessage `json:"payload"`
}

// Stuff is a sample struct for testing
type Stuff struct {
	Msg   string    `json:"msg"`
	Count int       `json:"count"`
	TS    time.Time `json:"timestamp"`
}

// FakeLoop is a sample Actionable function for testing
func FakeLoop() (chan interface{}, chan Results) {

	getter := make(chan interface{})
	teller := make(chan Results)

	go func() {
		var i int
		for {
			i++

			getter <- Stuff{
				Msg:   fmt.Sprintf("msg number: %d", i),
				Count: i,
				TS:    time.Now(),
			}
			results, ok := <-teller
			if !ok {
				fmt.Println("teller must be closed")
				break
			}
			if false && len(results.ErrMsg) > 0 {
				fmt.Println("Fakeloop got error:", results.ErrMsg)
			}
		}
		log.Println("FakeLoop is closing")
		close(getter)
	}()

	return getter, teller
}
