package httpticker

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"time"
)

var (
	ErrMissingURL        = errors.New("Missing URL")
	ErrMissingHandleFunc = errors.New("Missing handle func")
	NullURL              = "http://0.0.0.0:0000"

	DefaultTickerInterval = 500 * time.Millisecond
	DefaultMaxWorkers     = 10
)

type (
	HandleFunc  func(*http.Response)
	PoolChannel chan chan *http.Response

	Ticker struct {
		chief            Chief
		isHandleFuncSet  bool
		HandleFunc       HandleFunc
		MaxWorkers       int
		Url              string
		Ticker           *time.Ticker
		TickInterval     time.Duration
		HTTPClient       *http.Client
		HTTPRequest      *http.Request
		OnErrorRequests  []func(*Ticker, *http.Response, error)
		OnAfterResponses []func(*Ticker, *http.Response)
		Quit             chan bool
	}

	Worker struct {
		Job        chan *http.Response
		Pool       PoolChannel
		HandleFunc HandleFunc
		Quit       chan bool
	}

	Chief struct {
		Response   chan *http.Response
		Pool       PoolChannel
		HandleFunc HandleFunc
		Workers    []Worker
		MaxWorkers int
		Quit       chan bool
	}
)

func New() *Ticker {

	r, _ := http.NewRequest("GET", NullURL, bytes.NewBuffer([]byte{}))

	return &Ticker{
		isHandleFuncSet: false,
		MaxWorkers:      DefaultMaxWorkers,
		TickInterval:    DefaultTickerInterval,
		Ticker:          time.NewTicker(DefaultTickerInterval),
		HTTPClient:      http.DefaultClient,
		HTTPRequest:     r,
		Quit:            make(chan bool),
	}
}

func (t *Ticker) SetMaxWorkers(max int) *Ticker {
	t.MaxWorkers = max
	return t
}

func (t *Ticker) SetUrl(url string) *Ticker {
	t.Url = url
	req, err := http.NewRequest("GET", url, bytes.NewBuffer([]byte{}))
	if err != nil {
		log.Fatal(err)
	}
	t.HTTPRequest = req

	return t
}

func (t *Ticker) SetHandleFunc(fn HandleFunc) *Ticker {
	t.isHandleFuncSet = true
	t.HandleFunc = fn
	return t
}

func (t *Ticker) SetInterval(i time.Duration) *Ticker {
	t.TickInterval = i
	t.Ticker = time.NewTicker(i)
	return t
}

func (t *Ticker) SetHTTPClient(c *http.Client) *Ticker {
	t.HTTPClient = c
	return t
}

func (t *Ticker) SetHTTPRequest(r *http.Request) *Ticker {
	t.HTTPRequest = r
	return t
}

func (t *Ticker) OnErrorRequest(fn func(*Ticker, *http.Response, error)) *Ticker {
	t.OnErrorRequests = append(t.OnErrorRequests, fn)
	return t
}

func (t *Ticker) OnAfterResponse(fn func(*Ticker, *http.Response)) *Ticker {
	t.OnAfterResponses = append(t.OnAfterResponses, fn)
	return t
}

func (t *Ticker) Tick() error {
	if t.HTTPRequest.URL.String() == NullURL {
		return ErrMissingURL
	}

	if !t.isHandleFuncSet {
		return ErrMissingHandleFunc
	}

	c, resp := NewChief(t.MaxWorkers, t.HandleFunc)
	t.chief = c
	c.Start()

	t.tick(resp)

	return nil
}

func (t *Ticker) Stop() {
	t.Ticker.Stop()
	t.Quit <- true
	t.chief.Stop()
}

func (t *Ticker) tick(chief chan *http.Response) {
	for {
		select {
		case <-t.Ticker.C:
			resp, err := t.HTTPClient.Do(t.HTTPRequest)
			if err != nil || resp.StatusCode >= 500 {
				for _, errorHook := range t.OnErrorRequests {
					errorHook(t, resp, err)
				}
				continue
			}

			for _, afterResponseHook := range t.OnAfterResponses {
				afterResponseHook(t, resp)
			}
			chief <- resp
		case <-t.Quit:
			return
		}
	}
}

func NewChief(max int, fn HandleFunc) (Chief, chan *http.Response) {
	resp := make(chan *http.Response)
	return Chief{
		Pool:       make(PoolChannel),
		Response:   resp,
		HandleFunc: fn,
		Workers:    []Worker{},
		MaxWorkers: max,
		Quit:       make(chan bool),
	}, resp
}

func (c Chief) Start() {
	for x := 0; x < c.MaxWorkers; x++ {
		w := NewWorker(c.Pool, c.HandleFunc)
		c.Workers = append(c.Workers, w)
		w.Start()
	}

	go c.ctrl()
}

func (c Chief) Stop() {
	for _, w := range c.Workers {
		w.Stop()
	}
	c.Quit <- true
}

func (c Chief) ctrl() {
	for {
		select {
		case resp := <-c.Response:
			go func(r *http.Response) {
				worker := <-c.Pool
				worker <- r
			}(resp)

		case <-c.Quit:
			return
		}
	}
}

func NewWorker(pool PoolChannel, fn HandleFunc) Worker {
	return Worker{
		Pool:       pool,
		Job:        make(chan *http.Response),
		HandleFunc: fn,
		Quit:       make(chan bool),
	}

}

func (w Worker) Start() {
	go func() {
		for {
			w.Pool <- w.Job

			select {
			case resp := <-w.Job:
				w.HandleFunc(resp)

			case <-w.Quit:
				return
			}
		}
	}()
}

func (w Worker) Stop() {
	w.Quit <- true
}
