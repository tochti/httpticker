package httpticker

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_TickSetURLSetHandlerGetRequestDefaultClient(t *testing.T) {
	ts := createTestServer(func(resp http.ResponseWriter, req *http.Request) {})

	err := make(chan bool)
	boom := New().SetUrl(ts.URL)
	boom.SetHandleFunc(func(resp *http.Response) {
		err <- false
	})
	go boom.Tick()

	if <-err {
		t.Fatalf("Expect to send get request to %v", ts.URL)
	}

}

func Test_MissingURL(t *testing.T) {
	boom := New()
	err := boom.Tick()
	if err != ErrMissingURL {
		t.Fatalf("Expect %v was %v", ErrMissingURL, err)
	}
}

func Test_MissingHandleFunc(t *testing.T) {
	boom := New()
	err := boom.SetUrl("http://localhost").Tick()
	if err != ErrMissingHandleFunc {
		t.Fatalf("Expect %v was %v", ErrMissingHandleFunc, err)
	}
}

func Test_SetInterval(t *testing.T) {
	done := make(chan bool)
	count := 0
	timer := time.AfterFunc(100*time.Millisecond, func() {
		t.Fatal("Expect to receive a request")
	})
	ts := createTestServer(func(resp http.ResponseWriter, req *http.Request) {
		if count > 4 {
			timer.Stop()
			done <- true
		} else {
			count++
		}
		timer.Reset(100 * time.Millisecond)
	})

	boom := New().SetUrl(ts.URL).SetHandleFunc(func(*http.Response) {})
	boom.SetInterval(90 * time.Millisecond)
	go boom.Tick()

	<-done
}

func Test_TickSetRequest(t *testing.T) {
	expectReq := new(http.Request)
	xTestValue := "love"

	done := make(chan error)
	ts := createTestServer(func(resp http.ResponseWriter, req *http.Request) {
		v := req.Header.Get("X-TEST")
		if v != xTestValue {
			msg := "Expect value of X-TEST to be %v was %v"
			done <- errors.New(fmt.Sprintf(msg, xTestValue, v))
		}

		if req.Method != "POST" {
			msg := fmt.Sprintf("Expect method %v was %v", "POST", req.Method)
			done <- errors.New(msg)
		}

		done <- nil
	})

	expectReq, err := http.NewRequest("POST", ts.URL, bytes.NewBuffer([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	expectReq.Header.Set("X-TEST", xTestValue)

	boom := New().SetHTTPRequest(expectReq)
	boom.SetHandleFunc(func(resp *http.Response) {})
	go boom.Tick()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func Test_StartWorker(t *testing.T) {
	done := make(chan bool)
	fn := func(_ *http.Response) {
		done <- true
	}

	pool := make(PoolChannel, 1)
	w := NewWorker(pool, fn)
	w.Start()

	w.Job <- &http.Response{}

	<-done
}

func Test_StartChief(t *testing.T) {
	done := make(chan bool)
	fn := func(_ *http.Response) {
		done <- true
	}

	c, resp := NewChief(2, fn)
	c.Start()

	resp <- &http.Response{}

	<-done
}

func createTestServer(fn http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(fn))
}
