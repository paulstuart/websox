package websox

import (
	"time"
)

// Stuff is a sample struct for testing
type Stuff struct {
	Msg   string    `json:"msg"`
	Count int       `json:"count"`
	TS    time.Time `json:"timestamp"`
}
