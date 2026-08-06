package captcha

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

const (
	DefaultRetries   = 5
	DefaultTimeout   = 180 * time.Second
	initialPollDelay = 5 * time.Second
	pollInterval     = 5 * time.Second
	maxPollFailures  = 3
	initialBackoff   = 2 * time.Second
	maxBackoff       = 30 * time.Second
)

// Solver wires a Client with the injectable clock/sleep used by tests to avoid real delays.
type Solver struct {
	Client *Client
	Sleep  func(context.Context, time.Duration) error
}

func NewSolver(client *Client) *Solver {
	return &Solver{Client: client, Sleep: sleepContext}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Result is the outcome of a successful Solve.
type Result struct {
	TaskID    int64
	Solution  map[string]any
	Cost      string
	Attempts  int
	ElapsedMS int64
}

// Options configures a single Solve call.
type Options struct {
	Retries int           // outer submit+poll cycles; default DefaultRetries, clamped [1,10]
	Timeout time.Duration // per-attempt poll budget; default DefaultTimeout, clamped [10s,600s]
}

func (o Options) normalize() Options {
	if o.Retries <= 0 {
		o.Retries = DefaultRetries
	}
	o.Retries = min(o.Retries, 10)
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	o.Timeout = min(max(o.Timeout, 10*time.Second), 600*time.Second)
	return o
}

// Solve submits task (which must include a "type" field) and polls until it is ready, retrying
// the whole submit+poll cycle up to opts.Retries times on retryable errors (transport failures,
// ERROR_NO_SLOT_AVAILABLE, ERROR_CAPTCHA_UNSOLVABLE, ...). It returns immediately on a fatal
// API error (bad key, zero balance, malformed task, ...) without consuming remaining attempts.
func (s *Solver) Solve(ctx context.Context, task map[string]any, opts Options) (*Result, error) {
	opts = opts.normalize()
	start := time.Now()

	var lastErr error
	for attempt := 1; attempt <= opts.Retries; attempt++ {
		result, err := s.attempt(ctx, task, opts.Timeout)
		if err == nil {
			result.Attempts = attempt
			result.ElapsedMS = time.Since(start).Milliseconds()
			return result, nil
		}
		lastErr = err
		if IsFatal(err) {
			return nil, fmt.Errorf("attempt %d/%d: %w", attempt, opts.Retries, err)
		}
		if attempt == opts.Retries {
			break
		}
		if sleepErr := s.Sleep(ctx, backoff(attempt)); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, fmt.Errorf("exhausted %d attempts, last error: %w", opts.Retries, lastErr)
}

// attempt runs one submit+poll cycle within budget.
func (s *Solver) attempt(ctx context.Context, task map[string]any, budget time.Duration) (*Result, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	taskID, err := s.Client.CreateTask(attemptCtx, task)
	if err != nil {
		return nil, err
	}

	if err := s.Sleep(attemptCtx, initialPollDelay); err != nil {
		return nil, err
	}

	pollFailures := 0
	for {
		res, err := s.Client.GetTaskResult(attemptCtx, taskID)
		if err != nil {
			if IsFatal(err) {
				return nil, err
			}
			pollFailures++
			if pollFailures >= maxPollFailures {
				return nil, err
			}
			if sleepErr := s.Sleep(attemptCtx, pollInterval); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}
		pollFailures = 0
		if res.Status == "ready" {
			return &Result{TaskID: taskID, Solution: res.Solution, Cost: res.Cost}, nil
		}
		if sleepErr := s.Sleep(attemptCtx, pollInterval); sleepErr != nil {
			return nil, sleepErr
		}
	}
}

// backoff returns an exponentially growing, jittered delay before retry attempt+1.
func backoff(attempt int) time.Duration {
	d := min(initialBackoff*time.Duration(1<<uint(attempt-1)), maxBackoff)
	jitter := time.Duration(rand.Int64N(int64(d) / 2))
	return d/2 + jitter
}

// Token extracts the convenience answer string from a solution using t's SolutionKeys, in order.
func Token(t TaskType, solution map[string]any) string {
	for _, key := range t.SolutionKeys {
		if v, ok := solution[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
