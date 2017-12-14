package websox

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	greeting, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%s", greeting)
}

func TestPusher(t *testing.T) {
	expires := time.Second * 86400
	ping := time.Second * 3600
	ts := httptest.NewServer(http.HandlerFunc(Pusher(FakeLoop, expires, ping)))
	//ts := httptest.NewServer(http.HandlerFunc(Pusher(fakeSend, expires, ping)))
	defer ts.Close()

	if err := Client(ts.URL, gotIt, true, nil); err != nil {
		t.Fatal(err)
	}
}

func fakeSend() (chan interface{}, chan Results) {
	var i int

	getter := make(chan interface{})
	teller := make(chan Results)

	go func() {
		getter <- Stuff{
			Msg:   fmt.Sprintf("msg number: %d", i),
			Count: i,
			TS:    time.Now(),
		}
		results, ok := <-teller
		if !ok {
			fmt.Println("teller must be closed")
		}
		if false && len(results.ErrMsg) > 0 {
			fmt.Println("fakeloop got error:", results.ErrMsg)
		}
		log.Println("fakeSend is closing")
		close(getter)
	}()

	return getter, teller
}

func gotIt(r io.Reader) (interface{}, bool, error) {
	var s Stuff
	ok := false
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		fmt.Println("json error:", err)
		return nil, false, err
	}
	fmt.Println("Stuff count:", s.Count)
	three := s.Count%3 == 0
	five := s.Count%5 == 0
	switch {
	case three && five:
		return "DA fizzbuzz", ok, fmt.Errorf("XX fizzbuzz")
	case three:
		return "fizz", ok, fmt.Errorf("fizz")
	case five:
		return "buzz", ok, fmt.Errorf("buzz")
	}
	return nil, ok, nil
}
