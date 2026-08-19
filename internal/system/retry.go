package system

import (
	"context"
	"time"
)

type RetryPolicy struct {
	Attempts int
	Delay    time.Duration
	MaxDelay time.Duration
}

func (p RetryPolicy) Normalize() RetryPolicy {
	if p.Attempts < 1 {
		p.Attempts = 1
	}
	if p.Delay <= 0 {
		p.Delay = time.Second
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 30 * time.Second
	}
	if p.MaxDelay < p.Delay {
		p.MaxDelay = p.Delay
	}
	return p
}

func (p RetryPolicy) Run(ctx context.Context, operation func(context.Context) error) error {
	p = p.Normalize()
	var lastErr error
	for attempt := 0; attempt < p.Attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = operation(ctx)
		if lastErr == nil {
			return nil
		}
		if attempt == p.Attempts-1 {
			break
		}
		delay := p.Delay << attempt
		if delay > p.MaxDelay {
			delay = p.MaxDelay
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}
