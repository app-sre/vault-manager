package utils

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		description string
		attempts    int
		sleep       time.Duration
		given       func(*int) func() error
		expected    error
	}{
		{
			"callback function returns no error",
			1,
			1 * time.Millisecond,
			func(c *int) func() error {
				return func() error {
					*c++
					return nil
				}
			},
			nil,
		},
		{
			"callback function returns an error",
			1,
			1 * time.Millisecond,
			func(c *int) func() error {
				return func() error {
					*c++
					return errors.New("test")
				}
			},
			errors.New("test"),
		},
		{
			"callback function returns an error to be retried five times",
			5,
			1 * time.Millisecond,
			func(c *int) func() error {
				return func() error {
					*c++
					return errors.New("test")
				}
			},
			errors.New("test"),
		},
		{
			"callback function returns no error using a custom error",
			1,
			1 * time.Millisecond,
			func(c *int) func() error {
				return func() error {
					*c++
					return RetryStop(nil)
				}
			},
			nil,
		},
		{
			"callback function requests to stop retrying with a custom error",
			1,
			1 * time.Millisecond,
			func(c *int) func() error {
				return func() error {
					*c++
					return RetryStop(errors.New("test-retry-stop"))
				}
			},
			errors.New("test-retry-stop"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			var attempts int
			actual := Retry(tc.attempts, tc.sleep, tc.given(&attempts))

			assert.Equal(t, tc.expected, actual)
			assert.Equal(t, tc.attempts, attempts)
		})
	}
}
