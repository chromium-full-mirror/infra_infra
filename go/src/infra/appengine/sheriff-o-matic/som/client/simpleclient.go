package client

import (
	"fmt"
	"math"
	"net/http"
	"time"
)

type simpleClient struct {
	Host   string
	Client *http.Client
}

var retryBaseDelay = time.Second

func retry(f func() (bool, error), maxAttempts int) error {
	for attempt := range maxAttempts {
		again, err := f()
		if !again {
			return err
		}

		if err == nil {
			return nil
		}

		time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * retryBaseDelay)
	}

	return fmt.Errorf("error max retries exceeded")
}
