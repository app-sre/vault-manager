package utils

import (
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

type retryError struct {
	error
}

// Retry attempts to execute the given callback function a certain number of times,
// with a delay between each attempt using a simple exponential backoff algorithm
// that uses a slight jitter to ensure that retries aren't clustered.
//
// If the callback function returns an error, Retry will sleep for a certain duration
// before attempting to call the callback function again.
//
// Retry will repeat this process until the callback function succeeds or until the
// maximum number of attempts is reached.
//
// If the maximum number of attempts is reached and the callback function still fails,
// it returns the last error returned by the callback function.
func Retry(attempts int, sleep time.Duration, callbackFunc func() error) error {
	if err := callbackFunc(); err != nil {
		log.WithField("attempts", attempts).Debug("Retrying attempt")
		if s, ok := err.(retryError); ok {
			return s.error
		}
		if attempts--; attempts > 0 {
			sleep = sleep + jitter(sleep)/2
			log.WithField("sleep", sleep).Debug("Sleeping before retrying")
			time.Sleep(sleep)
			return Retry(attempts, 2*sleep, callbackFunc)
		}
		return err
	}
	return nil
}

// RetryStop wraps the given error in a private retryError type and returns it to
// signal that a retry loop should stop attempting the operation that produced the
// given error.
func RetryStop(err error) error {
	return retryError{err}
}

// jitter returns a random duration that is less than the input duration to
// introduce randomness into the duration that a retry loop sleeps between
// retry attempts.
func jitter(t time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(t)))
}
