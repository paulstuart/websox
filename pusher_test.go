package websox

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestExpiresOk(t *testing.T) {
	expires := time.Duration(time.Second * 1)
	timeout := expires * 2

	sender := func() (chan io.ReadCloser, chan Results) {
		t.Log("sender started")
		getter := make(chan io.ReadCloser)
		teller := make(chan Results)
		go func() {
			t.Log("***** give to getter")
			getter <- Stuff{
				Msg:   "message for getter",
				Count: 1,
				TS:    time.Now(),
			}.ReadCloser()

			t.Log("***** wait for results")
			results, ok := <-teller
			t.Log("***** results:", results, "OK:", ok)
			if !ok {
				t.Fatal("teller was closed")
			}

			if len(results.ErrMsg) > 0 {
				t.Log("fakeloop got error:", results.ErrMsg)
			}

			t.Logf("sender is closing")
			close(getter)
		}()

		return getter, teller
	}
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, expires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, sleeper(logger, timeout), true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestExpiresTimeout(t *testing.T) {
	expires := time.Duration(time.Second * 1)
	timeout := expires * 2

	sender := func() (chan io.ReadCloser, chan Results) {
		t.Log("sender started")
		getter := make(chan io.ReadCloser)
		teller := make(chan Results)
		go func() {
			t.Log("***** give to getter")
			getter <- Stuff{
				Msg:   "message for getter",
				Count: 1,
				TS:    time.Now(),
			}.ReadCloser()

			t.Log("***** wait for results")
			results, ok := <-teller
			t.Log("***** results:", results, "OK:", ok)
			if !ok {
				t.Fatal("teller was closed")
			}

			if len(results.ErrMsg) > 0 {
				t.Log("fakeloop got error:", results.ErrMsg)
			}

			t.Logf("sender is closing")
			close(getter)
		}()

		return getter, teller
	}
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, expires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, sleeper(logger, timeout*2), true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestBadClient(t *testing.T) {
	expires := time.Duration(time.Second * 1)
	timeout := expires * 2

	sender := func() (chan io.ReadCloser, chan Results) {
		t.Log("sender started")
		getter := make(chan io.ReadCloser)
		teller := make(chan Results)
		go func() {
			t.Log("***** give to getter")
			getter <- Stuff{
				Msg:   "message for getter",
				Count: 1,
				TS:    time.Now(),
			}.ReadCloser()

			t.Log("***** wait for results")
			results, ok := <-teller
			t.Log("***** results:", results, "OK:", ok)
			if !ok {
				t.Fatal("teller was closed")
			}

			if len(results.ErrMsg) > 0 {
				t.Log("fakeloop got error:", results.ErrMsg)
			}

			t.Logf("sender is closing")
			close(getter)
		}()

		return getter, teller
	}
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, expires, testPing, nil, logger)))
	defer ts.Close()

	if err := badClient(ts.URL, logger, timeout); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func badClient(url string, logger *log.Logger, wait time.Duration) error {
	conn, err := dial(url, nil, logger, nil)
	if err != nil {
		return err
	}
	time.Sleep(wait)
	return conn.Close()
}

func TestBadResponse(t *testing.T) {
	expires := time.Duration(time.Second * 1)
	timeout := expires * 2

	sender := func() (chan io.ReadCloser, chan Results) {
		t.Log("sender started")
		getter := make(chan io.ReadCloser)
		teller := make(chan Results)
		go func() {
			t.Log("***** give to getter")
			getter <- Stuff{
				Msg:   "message for getter",
				Count: 1,
				TS:    time.Now(),
			}.ReadCloser()

			t.Log("***** wait for results")
			results, ok := <-teller
			t.Log("***** results:", results, "OK:", ok)
			if !ok {
				t.Fatal("teller was closed")
			}

			if len(results.ErrMsg) > 0 {
				t.Log("fakeloop got error:", results.ErrMsg)
			}

			t.Logf("sender is closing")
			close(getter)
		}()

		return getter, teller
	}
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, expires, testPing, nil, logger)))
	defer ts.Close()

	if err := badClient(ts.URL, logger, timeout); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func clientEmptyResponse(url string, logger *log.Logger, wait time.Duration) error {
	conn, err := dial(url, nil, logger, nil)
	if err != nil {
		return err
	}
	time.Sleep(wait)

	_, _, err = conn.NextReader()
	if err != nil {
		return err
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte{}); err != nil {
		return err
	}
	return conn.Close()
}
