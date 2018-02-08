// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

// ReadCloser is a convenience method for generating a ReadCloser representing the struct
func (s Stuff) ReadCloser() io.ReadCloser {
	var buff bytes.Buffer
	json.NewEncoder(&buff).Encode(s)
	return ioutil.NopCloser(&buff)
}

// MakeFake returns a sample Actionable function for testing
func MakeFake(logger *log.Logger) Setup {
	return func() (chan io.ReadCloser, chan Results) {

		getter := make(chan io.ReadCloser)
		teller := make(chan Results)

		go func() {
			var i int
			for {
				i++

				stuff := Stuff{
					Msg:   fmt.Sprintf("msg number: %d", i),
					Count: i,
					TS:    time.Now(),
				}
				getter <- stuff.ReadCloser()
				results, ok := <-teller
				if !ok {
					logger.Println("teller must be closed")
					break
				}
				if false && len(results.ErrMsg) > 0 {
					logger.Println("Fakeloop got error:", results.ErrMsg)
				}
			}
			logger.Println("FakeLoop is closing")
			close(getter)
		}()

		return getter, teller
	}
}
