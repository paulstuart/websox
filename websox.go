// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"fmt"
	"time"
)

// ErrMsg is for reporting errors on websocket pushes
type ErrMsg struct {
	Msg string `json:"errmsg"`
}

// Error returns an error if any error message is returned
func (e ErrMsg) Error() error {
	if len(e.Msg) == 0 {
		return nil
	}
	return fmt.Errorf(e.Msg)
}

// Stuff is a sample struct for testing
type Stuff struct {
	Msg   string    `json:"msg"`
	Count int       `json:"count"`
	TS    time.Time `json:"timestamp"`
}
