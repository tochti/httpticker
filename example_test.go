package httpticker

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

// Starte einen Ticker welcher alle 5 Sekunden eine Request an eine
// übergeben URL sendet. Die Antworten werden an 5 Workers verteilt
// welche die einkommenden Antworten vearbeiten.
func Example_Basic() {
	ticker := New()
	ticker.SetUrl("http://example").SetInterval(5 * time.Second)
	ticker.SetMaxWorkers(5)
	ticker.SetHandleFunc(func(resp *http.Response) {
		// Have Fun
	})

	err := ticker.Tick()
	if err != nil {
		log.Fatal(err)
	}
}

// Starte einen Ticker der jede Skunde eine Anfrage an das Telegram
// Netzwerk sendet.
func Example_Telegram() {
	token := "secret"
	client := &http.Client{Timeout: 5 * time.Millisecond}

	val := url.Values{}
	val.Set("offset", "-1")
	val.Add("limit", "20")
	val.Add("timeout", "10")
	url := fmt.Sprintf("https://api.telegram.org/bot%v/getUpdates?%v", token, val.Encode())

	emptyBody := bytes.NewBuffer([]byte{})
	request, err := http.NewRequest("GET", url, emptyBody)
	if err != nil {
		log.Fatal(err)
	}

	ticker := New()
	ticker.SetHTTPClient(client)
	ticker.SetHTTPRequest(request)
	ticker.SetHandleFunc(func(resp *http.Response) {
		// Dance with the wolfs
	})
	ticker.OnAfterResponse(func(ticker *Ticker, resp *http.Response) {
		// Update offset und update URL
	})
	ticker.OnErrorRequest(func(ticker *Ticker, resp *http.Response, err error) {
		// Setze Request Interval hoch
		ticker.SetInterval(5 * time.Second)
		log.Println(err)
	})
}
