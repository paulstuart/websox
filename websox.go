// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"time"
)

// Results is used to return client results / errors on websocket pushes
type Results struct {
	ErrMsg  string      `json:"error"`
	Payload interface{} `json:"payload"`
}

// Stuff is a sample struct for testing
type Stuff struct {
	Msg   string    `json:"msg"`
	Count int       `json:"count"`
	TS    time.Time `json:"timestamp"`
}
