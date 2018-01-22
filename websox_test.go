package websox

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var (
	testExpires = time.Second * 86400
	testPing    = time.Second * 3600
	testTimeout = time.Second * 1
	logger      = log.New(ioutil.Discard, "", logFlags)
)

func testingSetup() {
	if testing.Verbose() {
		logger.SetOutput(os.Stdout)
	}
}

// TestPusher just does a simple send
func TestPusher(t *testing.T) {
	expires := time.Second * 86400
	ping := time.Second * 3600
	ts := httptest.NewServer(http.HandlerFunc(Pusher(MakeFake(logger), expires, ping, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, gotIt, true, nil, logger); err != nil {
		t.Fatal(err)
	}
}

// TestSendFail tests if client fails
func TestSendFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sendFail(t), testExpires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, takeFail(t, 1, nil), true, nil, logger); err != nil {
		t.Fatal(err)
	}
}

func TestTenSend(t *testing.T) {
	const limit = 10

	var count int

	counter := func(r io.Reader) (interface{}, bool, error) {
		count++
		t.Log("COUNT:", count)
		return nil, true, nil
	}
	sender := func() (chan interface{}, chan Results) {
		t.Log("sender started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			for i := 0; i < limit; i++ {
				getter <- Stuff{
					Msg:   fmt.Sprintf("msg number: %d", i),
					Count: i,
					TS:    time.Now(),
				}
				results, ok := <-teller
				if !ok {
					t.Fatal("teller must be closed")
					break
				}

				if len(results.ErrMsg) > 0 {
					t.Log("sender got error:", results.ErrMsg)
				}
			}
			close(getter)
		}()

		return getter, teller
	}

	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, counter, true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
	if count != limit {
		t.Fatalf("expected: %d but got: %d", limit, count)
	}
}

func TestTenToOne(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sendX(t, 10), testExpires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, gotIt, true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestFivers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sendX(t, 10), testExpires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, takeX(t, 5, nil), true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
	if err := Client(ts.URL, takeX(t, 5, nil), true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
	if err := Client(ts.URL, takeX(t, 5, nil), true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func sendX(t *testing.T, limit int) Setup {
	t.Log("sendX invoked, limit:", limit)
	return func() (chan interface{}, chan Results) {
		t.Log("sendX started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			for i := 0; i < limit; i++ {
				t.Log("***** sendX to getter")
				getter <- Stuff{
					Msg:   fmt.Sprintf("msg number: %d", i),
					Count: i,
					TS:    time.Now(),
				}
				t.Log("***** sendX wait for results")
				results, ok := <-teller
				t.Log("***** sendX results:", results, "OK:", ok)
				if !ok {
					//t.Fatal("teller must be closed")
					break
				}
				t.Log("***** sendX continues")

				if len(results.ErrMsg) > 0 {
					t.Log("fakeloop got error:", results.ErrMsg)
				}
				t.Log("***** sendX end of loop")
			}
			t.Logf("sendX is closing")
			close(getter)
		}()

		return getter, teller
	}
}

func sendFail(t *testing.T) Setup {
	return func() (chan interface{}, chan Results) {
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			t.Log("sending Stuff")
			getter <- Stuff{
				Msg:   fmt.Sprintf("msg number: %d", 1),
				Count: 1,
				TS:    time.Now(),
			}
			t.Log("sent Stuff")
			results, ok := <-teller
			t.Log("sent results:", results)
			if !ok {
				t.Log("teller must be closed")
			}
			if len(results.ErrMsg) > 0 {
				t.Log("sendFail got error:", results.ErrMsg)
			}
			t.Logf("sendFail is closing")
			close(getter)
		}()

		return getter, teller
	}
}

func gotIt(r io.Reader) (interface{}, bool, error) {
	var s Stuff
	ok := false
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		fmt.Println("json error:", err)
		return nil, false, err
	}
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

func takeX(t *testing.T, x int, err error) Actionable {
	count := 0
	return func(r io.Reader) (interface{}, bool, error) {
		count++
		t.Log("takeX:", count)
		ok := true
		if count <= x {
			ok = false
			t.Logf("reached max count: %d", x)
		}

		return count, ok, err
	}
}

func takeFail(t *testing.T, x int, err error) Actionable {
	count := 0
	return func(r io.Reader) (interface{}, bool, error) {
		count++
		t.Log("takeX:", count)
		ok := true
		if count <= x {
			ok = false
			t.Logf("reached max count: %d", x)
		}

		return badjson{}, ok, err
	}
}

func TestNoAnswer(t *testing.T) {

	var received bool

	receiver := func(r io.Reader) (interface{}, bool, error) {
		received = true
		return fmt.Errorf("received but not filed"), false, fmt.Errorf("receiver error")
	}

	sender := func() (chan interface{}, chan Results) {
		t.Log("noAnswer started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			t.Log("***** noAnswer begins")
			getter <- Stuff{
				Msg:   fmt.Sprintf("msg number: %d", 1),
				Count: 1,
				TS:    time.Now(),
			}
			t.Log("***** noAnswer wait for results")
			results, ok := <-teller
			t.Log("***** noAnswer TELLER results:", results, "OK:", ok)
			if !ok {
				fmt.Println("teller must be closed")
				t.Fatal("teller must be closed")
			}
			t.Log("noAnswer got error:", results.ErrMsg)
			close(getter)
		}()

		return getter, teller
	}

	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, receiver, true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func badHandler(conn *websocket.Conn, fn Actionable) error {
	log.Println("===> badHandler waiting for message")
	defer log.Println("===> badHandler exiting")
	_, _, err := conn.NextReader()
	if err != nil {
		if err != nil && websocket.IsCloseError(err, 1000) {
			return nil
		}
		log.Println("NextReader error:", err)
		return err
	}
	return nil

}

// badjson is a struct designed to be unmarshalable and trigger errors
type badjson struct{}

func (b badjson) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("this is a bad JSON error")
}

func badAction(r io.Reader) (interface{}, bool, error) {
	return badjson{}, false, fmt.Errorf("badAction error")
}

func TestBadAction(t *testing.T) {
	sender := func() (chan interface{}, chan Results) {
		t.Log("**BadAction** started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			t.Log("***** **BadAction** begins")
			getter <- Stuff{
				Msg:   fmt.Sprintf("msg number: %d", 1),
				Count: 1,
				TS:    time.Now(),
			}
			t.Log("***** **BadAction** wait for results")
			results, ok := <-teller
			t.Log("***** **BadAction** TELLER results:", results, "OK:", ok)
			if !ok {
				t.Log("teller must be closed")
			}
			t.Log("**BadAction** got error:", results.ErrMsg)
			close(getter)
		}()

		return getter, teller
	}

	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, badAction, true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestSingleOk(t *testing.T) {

	const limit = 10
	counter := 0

	receiver := func(r io.Reader) (interface{}, bool, error) {
		counter++
		return nil, false, nil
	}

	done := make(chan bool)
	sender := func() (chan interface{}, chan Results) {
		t.Log("SingleOk started")
		getter := make(chan interface{}, 1)
		teller := make(chan Results)
		go func() {
			for i := 0; i < limit; i++ {
				getter <- Stuff{
					Msg:   fmt.Sprintf("msg number: %d", i),
					Count: i,
					TS:    time.Now(),
				}
				_, ok := <-teller
				if !ok {
					break
				}
			}
			t.Logf("SingleOk is closing")
			close(getter)
			done <- true
		}()

		return getter, teller
	}

	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, receiver, true, nil, logger); err != nil {
		t.Fatal("unexpected error:", err)
	}

	<-done
	if counter != 1 {
		t.Fatalf("counter is: %d -- expected: %d", counter, 1)
	}
	t.Log("SingleOk complete")
}

// TestTimeout tests handling of a connection close because of session timeout
func TestTimeout(t *testing.T) {
	expires := time.Second * 5
	ping := time.Second * 3600
	ts := httptest.NewServer(http.HandlerFunc(Pusher(MakeFake(logger), expires, ping, nil, logger)))
	defer ts.Close()

	if err := Client(ts.URL, sleeper(logger, time.Second*10), true, nil, logger); err != nil {
		t.Fatal(err)
	}
}

func sleeper(logger *log.Logger, d time.Duration) func(r io.Reader) (interface{}, bool, error) {
	return func(r io.Reader) (interface{}, bool, error) {
		var s Stuff
		ok := false
		if err := json.NewDecoder(r).Decode(&s); err != nil {
			fmt.Println("json error:", err)
			return nil, false, err
		}
		logger.Println("sleep for:", d)
		time.Sleep(d)
		logger.Println("done sleeping")
		return nil, ok, nil
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	testingSetup()
	retCode := m.Run()
	//testingTeardown()
	os.Exit(retCode)
}
