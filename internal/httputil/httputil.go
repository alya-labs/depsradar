package httputil

import (
	"math"
	"net/http"
	"time"
)

const (
	MaxRetries = 3
	BaseDelay  = 500 * time.Millisecond
)

// DoWithRetry executes an HTTP request with exponential backoff retry.
// It retries on 5xx status codes and network errors.
func DoWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(float64(BaseDelay) * math.Pow(2, float64(attempt-1)))
			time.Sleep(delay)
		}

		resp, err = client.Do(req)
		if err != nil {
			continue
		}

		// Retry on server errors
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return resp, err
}
