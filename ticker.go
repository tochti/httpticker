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
		isHandleFuncSet bool
		HandleFunc      HandleFunc
		MaxWorkers      int
		Url             string
		Ticker          *time.Ticker
		TickInterval    time.Duration
		HTTPClient      *http.Client
		HTTPRequest     *http.Request
	}

	Worker struct {
		Job        chan *http.Response
		Pool       PoolChannel
		HandleFunc HandleFunc
	}

	Chief struct {
		Response   chan *http.Response
		Pool       PoolChannel
		HandleFunc HandleFunc
		MaxWorkers int
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
		log.Println(err)
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

func (t *Ticker) Tick() error {
	if t.HTTPRequest.URL.String() == NullURL {
		return ErrMissingURL
	}

	if !t.isHandleFuncSet {
		return ErrMissingHandleFunc
	}

	c, resp := NewChief(t.MaxWorkers, t.HandleFunc)
	c.Start()

	t.tick(resp)

	return nil
}

func (t Ticker) tick(chief chan *http.Response) {
	for {
		select {
		case <-t.Ticker.C:
			resp, err := t.HTTPClient.Do(t.HTTPRequest)
			if err != nil {
				log.Println(err)
				continue
			}
			chief <- resp
		}
	}
}

func NewChief(max int, fn HandleFunc) (Chief, chan *http.Response) {
	resp := make(chan *http.Response)
	return Chief{
		Pool:       make(PoolChannel),
		Response:   resp,
		HandleFunc: fn,
		MaxWorkers: max,
	}, resp
}

func (c Chief) Start() {
	for x := 0; x < c.MaxWorkers; x++ {
		w := NewWorker(c.Pool, c.HandleFunc)
		w.Start()
	}

	go c.ctrl()
}

func (c Chief) ctrl() {
	for {
		select {
		case resp := <-c.Response:
			go func(r *http.Response) {
				worker := <-c.Pool
				worker <- r
			}(resp)
		}
	}
}

func NewWorker(pool PoolChannel, fn HandleFunc) Worker {
	return Worker{
		Pool:       pool,
		Job:        make(chan *http.Response),
		HandleFunc: fn,
	}

}

func (w Worker) Start() {
	go func() {
		for {
			w.Pool <- w.Job

			select {
			case resp := <-w.Job:
				w.HandleFunc(resp)

			}
		}
	}()
}
