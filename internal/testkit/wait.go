package testkit

import (
	"fmt"
	"time"
)

type WaitUntilOptions struct {
	MaxAttempts int
	Interval    time.Duration
	OnRetry     func(attempt int)
}

func WaitUntil(opts WaitUntilOptions, condition func() (bool, error)) error {
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ok, err := condition()
		if ok && err == nil {
			return nil
		}
		if err != nil {
			lastErr = err
		}
		if attempt == maxAttempts {
			break
		}
		if opts.OnRetry != nil {
			opts.OnRetry(attempt)
		}
		time.Sleep(interval)
	}

	if lastErr != nil {
		return fmt.Errorf("condition not met after %d attempts: %w", maxAttempts, lastErr)
	}
	return fmt.Errorf("condition not met after %d attempts", maxAttempts)
}
