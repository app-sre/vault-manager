package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	log "github.com/sirupsen/logrus"
)

type retryError struct {
	error
}

const (
	// The factor by which to helve the jitter value.
	jitterHelveFactor = 2

	// The factor by which to increase the sleep time.
	sleepStepFactor = 2
)

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
	var e retryError

	if err := callbackFunc(); err != nil {
		log.WithField("attempts", attempts).Debug("Retrying attempt")
		if errors.As(err, &e) {
			return e.error
		}
		if attempts--; attempts > 0 {
			sleep += jitter(sleep) / jitterHelveFactor
			log.WithField("sleep", sleep).Debug("Sleeping before retrying")
			time.Sleep(sleep)
			return Retry(attempts, sleepStepFactor*sleep, callbackFunc)
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
	n, err := rand.Int(rand.Reader, big.NewInt(int64(t)))
	if err != nil {
		// Hopefully, this would never happen.
		panic(fmt.Sprintf("unable to read random bytes: %s", err.Error()))
	}
	return time.Duration(n.Int64())
}
