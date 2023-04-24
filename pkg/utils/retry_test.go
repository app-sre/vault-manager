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
		given       func() error
		expected    error
	}{
		{
			"callback returns no error",
			2,
			1 * time.Millisecond,
			func() error {
				return nil
			},
			nil,
		},
		{
			"callback returns an error",
			2,
			1 * time.Millisecond,
			func() error {
				return errors.New("test")
			},
			errors.New("test"),
		},
		{
			"callback returns no error using a custom error",
			2,
			1 * time.Millisecond,
			func() error {
				return RetryStop(nil)
			},
			nil,
		},
		{
			"callback requests to stop retrying with a custom error",
			2,
			1 * time.Millisecond,
			func() error {
				return RetryStop(errors.New("test-retry-stop"))
			},
			errors.New("test-retry-stop"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			actual := Retry(tc.attempts, tc.sleep, tc.given)

			assert.Equal(t, tc.expected, actual)
		})
	}
}
