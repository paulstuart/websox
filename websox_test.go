package websox

import (
	"encoding/json"
	"fmt"
	"io"
	//	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var (
	testExpires = time.Second * 86400
	testPing    = time.Second * 3600
)

/*
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
*/

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

var expected = fmt.Errorf("this error was expected")

func TestSendFail(t *testing.T) {
	//ts := httptest.NewServer(http.HandlerFunc(Pusher(FakeLoop, testExpires, testPing)))
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sendFail(t), testExpires, testPing)))
	defer ts.Close()

	if err := Client(ts.URL, takeFail(t, 1, nil), true, nil); err != nil {
		t.Fatal(err)
	}
	/*
			if err == nil {
				t.Fatal("expected an error")
			}
		if err != expected {
			t.Fatalf("expected: %v but got: %v", expected, err)
		}
	*/
}

/*
func xTestClientExitError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(Pusher(FakeLoop, testExpires, testPing)))
	defer ts.Close()

	err := Client(ts.URL, gotErr, true, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if err != expected {
		t.Fatalf("expected: %v but got: %v", expected, err)
	}
}
*/

func gotErr(r io.Reader) (interface{}, bool, error) {
	return nil, false, expected
}

func TestTenSend(t *testing.T) {
	const limit = 10

	var count int

	counter := func(r io.Reader) (interface{}, bool, error) {
		count++
		fmt.Println("GOT COUNT:", count)
		t.Log("COUNT:", count)
		return nil, true, nil
	}
	sender := func() (chan interface{}, chan Results) {
		t.Log("sendX started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			fmt.Println("***** sendX limit:", limit)
			for i := 0; i < limit; i++ {
				fmt.Println("***** sendX to getter")
				getter <- Stuff{
					Msg:   fmt.Sprintf("msg number: %d", i),
					Count: i,
					TS:    time.Now(),
				}
				fmt.Println("***** sendX wait for results")
				results, ok := <-teller
				fmt.Println("***** sendX results:", results, "OK:", ok)
				if !ok {
					t.Fatal("teller must be closed")
					break
				}
				fmt.Println("***** sendX continues")

				if len(results.ErrMsg) > 0 {
					t.Log("fakeloop got error:", results.ErrMsg)
				}
				fmt.Println("***** sendX end of loop")
			}
			fmt.Println("***** sendX is closing")
			t.Logf("sendX is closing")
			close(getter)
		}()

		return getter, teller
	}

	//ts := httptest.NewServer(http.HandlerFunc(Pusher(sendX(t, limit), testExpires, testPing)))
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing)))
	defer ts.Close()

	if err := Client(ts.URL, counter, true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}
	/*
		if websocket.IsUnexpectedCloseError(err, 1000) {
			t.Fatalf("unexpected error: %v", err)
			log.Println("websocket close error:", err)
		}
	*/
	if count != limit {
		t.Fatalf("expected: %d but got: %d", limit, count)
	}
}

func TestTenToOne(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sendX(t, 10), testExpires, testPing)))
	defer ts.Close()

	if err := Client(ts.URL, gotIt, true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestFivers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sendX(t, 10), testExpires, testPing)))
	defer ts.Close()

	if err := Client(ts.URL, takeX(t, 5, nil), true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}
	if err := Client(ts.URL, takeX(t, 5, nil), true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}
	if err := Client(ts.URL, takeX(t, 5, nil), true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func sendX(t *testing.T, limit int) Setup {
	t.Log("sendX invoked")
	fmt.Println("***** sendX invoked")
	return func() (chan interface{}, chan Results) {
		t.Log("sendX started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			fmt.Println("***** sendX limit:", limit)
			for i := 0; i < limit; i++ {
				fmt.Println("***** sendX to getter")
				getter <- Stuff{
					Msg:   fmt.Sprintf("msg number: %d", i),
					Count: i,
					TS:    time.Now(),
				}
				fmt.Println("***** sendX wait for results")
				results, ok := <-teller
				fmt.Println("***** sendX results:", results, "OK:", ok)
				if !ok {
					t.Fatal("teller must be closed")
					break
				}
				fmt.Println("***** sendX continues")

				if len(results.ErrMsg) > 0 {
					t.Log("fakeloop got error:", results.ErrMsg)
				}
				fmt.Println("***** sendX end of loop")
			}
			fmt.Println("***** sendX is closing")
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

/*
func sendTen() (chan interface{}, chan Results) {

	getter := make(chan interface{})
	teller := make(chan Results)

	go func() {
		for i := 0; i < 10; i++ {
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
		}
		log.Println("sendTen is closing")
		close(getter)
	}()

	return getter, teller
}
*/

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

type Bad struct{}

func (b Bad) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("this is Bad")
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

		return Bad{}, ok, err
	}
}

func TestNoAnswer(t *testing.T) {

	var received bool

	receiver := func(r io.Reader) (interface{}, bool, error) {
		received = true
		fmt.Println("RECEIVED")
		return fmt.Errorf("received and filed"), false, fmt.Errorf("forced error")
	}
	sender := func() (chan interface{}, chan Results) {
		t.Log("noAnswer started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			fmt.Println("***** noAnswer begins")
			fmt.Println("***** noAnswer to getter")
			getter <- Stuff{
				Msg:   fmt.Sprintf("msg number: %d", 1),
				Count: 1,
				TS:    time.Now(),
			}
			fmt.Println("***** noAnswer wait for results")
			results, ok := <-teller
			fmt.Println("***** noAnswer TELLER results:", results, "OK:", ok)
			if !ok {
				fmt.Println("teller must be closed")
				t.Fatal("teller must be closed")
			}
			fmt.Println("noAnswer got error:", results.ErrMsg)

			fmt.Println("***** noAnswer is closing")
			t.Logf("noAnswer is closing")
			close(getter)
		}()

		return getter, teller
	}

	//ts := httptest.NewServer(http.HandlerFunc(Pusher(sendX(t, limit), testExpires, testPing)))
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing)))
	defer ts.Close()

	if err := BadClient(ts.URL, receiver, true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}
	/*
			if websocket.IsUnexpectedCloseError(err, 1000) {
				t.Fatalf("unexpected error: %v", err)
				log.Println("websocket close error:", err)
			}
		if count != limit {
			t.Fatalf("expected: %d but got: %d", limit, count)
		}
	*/
}

var BadClient = makeClient(badHandler)

/*
// BadClient will connect to url and apply the Actionable function to each message recieved
func BadClient(url string, fn Actionable, pings bool, headers http.Header) error {
	if strings.HasPrefix(url, "http") {
		url = "ws" + url[4:]
	}
	log.Printf("connecting to %s", url)
	conn, resp, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		if resp != nil {
			if resp.Body != nil {
				io.Copy(os.Stdout, resp.Body)
			}
			return errors.Wrapf(err, "dial code:%d status:%s", resp.StatusCode, resp.Status)
		}
		return errors.Wrap(err, "websocket dial error for url: "+url)
	}

	log.Printf("connected to %s", url)

	log.Println("client waiting for message")
	messageType, r, err := conn.NextReader()
	if err != nil {
		if err != nil && websocket.IsCloseError(err, 1000) {
			return nil
		}
		log.Println("NextReader error:", err)
		return err
	}

	switch messageType {
	case websocket.CloseMessage:
		log.Println("uhoh! closing time!")
		return nil
	case websocket.PingMessage:
		log.Println("PING!")
		return nil
	case websocket.PongMessage:
		log.Println("PONG!")
		return nil
	}

	if messageType != websocket.BinaryMessage {
		log.Printf("UNKNOWN MSG (%T):", messageType)
		io.Copy(os.Stdout, r)
		continue
	}

	// decompress the message before the function sees it
	var reply interface{}
	z, err := zlib.NewReader(r)
	if err == nil {
		reply, ok, err = fn(z)
		if err != nil {
			log.Println("ok:", ok, "fn err:", err)
		}
	}
	z.Close()
	fmt.Printf("=====> REPLY (%T): %v\n", reply, reply)

	fmt.Println("client returning error:", err)
	return err
}
*/

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

type badjson struct{}

func (b badjson) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("this is a bad JSON error")
}

func badAction(r io.Reader) (interface{}, bool, error) {
	fmt.Println("BADACTION RECEIVED")
	return badjson{}, false, fmt.Errorf("badAction error")
}

func TestBadAction(t *testing.T) {
	sender := func() (chan interface{}, chan Results) {
		t.Log("**BadAction** started")
		getter := make(chan interface{})
		teller := make(chan Results)
		go func() {
			fmt.Println("***** **BadAction** begins")
			fmt.Println("***** **BadAction** to getter")
			getter <- Stuff{
				Msg:   fmt.Sprintf("msg number: %d", 1),
				Count: 1,
				TS:    time.Now(),
			}
			fmt.Println("***** **BadAction** wait for results")
			results, ok := <-teller
			fmt.Println("***** **BadAction** TELLER results:", results, "OK:", ok)
			if !ok {
				fmt.Println("teller must be closed")
				t.Fatal("teller must be closed")
			}
			fmt.Println("**BadAction** got error:", results.ErrMsg)

			fmt.Println("***** **BadAction** is closing")
			t.Logf("**BadAction** is closing")
			close(getter)
		}()

		return getter, teller
	}

	//ts := httptest.NewServer(http.HandlerFunc(Pusher(sendX(t, limit), testExpires, testPing)))
	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing)))
	defer ts.Close()

	if err := Client(ts.URL, badAction, true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}
	fmt.Println("BadAction complete")
}

// Client takes one record w/o errors

func TestSingleOk(t *testing.T) {

	const limit = 10
	counter := 0

	receiver := func(r io.Reader) (interface{}, bool, error) {
		counter++
		fmt.Println("RECEIVED")
		return nil, false, nil
	}

	done := make(chan bool)
	sender := func() (chan interface{}, chan Results) {
		t.Log("SINGLE started")
		getter := make(chan interface{}, 1)
		teller := make(chan Results)
		go func() {
			fmt.Println("***** SINGLE begins")
			for i := 0; i < limit; i++ {
				fmt.Println("***** SINGLE to getter")
				getter <- Stuff{
					Msg:   fmt.Sprintf("msg number: %d", i),
					Count: i,
					TS:    time.Now(),
				}
				fmt.Println("***** SINGLE wait for results")
				results, ok := <-teller
				fmt.Println("***** SINGLE TELLER results:", results, "OK:", ok)
				if !ok {
					fmt.Println("teller must be closed")
					break
				}
				fmt.Println("SINGLE got error:", results.ErrMsg)
			}
			fmt.Println("***** SINGLE is closing")
			t.Logf("SINGLE is closing")
			close(getter)
			done <- true
		}()

		return getter, teller
	}

	ts := httptest.NewServer(http.HandlerFunc(Pusher(sender, testExpires, testPing)))
	defer ts.Close()

	if err := Client(ts.URL, receiver, true, nil); err != nil {
		t.Fatal("unexpected error:", err)
	}

	<-done
	if counter != 1 {
		t.Fatalf("counter is: %d -- expected: %d", counter, 1)
	}
	fmt.Println("SINGLE complete")
}
