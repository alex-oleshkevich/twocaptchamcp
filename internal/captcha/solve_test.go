package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fakeServer simulates the 2captcha JSON API v2. behavior is called for each createTask/
// getTaskResult request in sequence and returns the response body to send.
type fakeServer struct {
	mux         *http.ServeMux
	createCalls atomic.Int32
	resultCalls atomic.Int32
	createReply func(n int32) map[string]any
	resultReply func(n int32) map[string]any
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	fs := &fakeServer{mux: http.NewServeMux()}
	fs.mux.HandleFunc("/createTask", func(w http.ResponseWriter, r *http.Request) {
		n := fs.createCalls.Add(1)
		_ = json.NewEncoder(w).Encode(fs.createReply(n))
	})
	fs.mux.HandleFunc("/getTaskResult", func(w http.ResponseWriter, r *http.Request) {
		n := fs.resultCalls.Add(1)
		_ = json.NewEncoder(w).Encode(fs.resultReply(n))
	})
	return fs
}

func newTestSolver(t *testing.T, fs *fakeServer) *Solver {
	t.Helper()
	srv := httptest.NewServer(fs.mux)
	t.Cleanup(srv.Close)
	client := &Client{APIKey: "test", BaseURL: srv.URL, HTTPClient: srv.Client()}
	solver := NewSolver(client)
	solver.Sleep = func(context.Context, time.Duration) error { return nil } // no real waiting in tests
	return solver
}

func TestSolveSucceedsOnFirstAttempt(t *testing.T) {
	fs := newFakeServer(t)
	fs.createReply = func(int32) map[string]any { return map[string]any{"errorId": 0, "taskId": 42} }
	fs.resultReply = func(int32) map[string]any {
		return map[string]any{"errorId": 0, "status": "ready", "solution": map[string]any{"token": "abc"}, "cost": "0.001"}
	}
	solver := newTestSolver(t, fs)

	result, err := solver.Solve(context.Background(), map[string]any{"type": "TurnstileTaskProxyless"}, Options{})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}
	if result.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", result.Attempts)
	}
	if result.Solution["token"] != "abc" {
		t.Errorf("Solution[token] = %v, want abc", result.Solution["token"])
	}
}

func TestSolvePollsUntilReady(t *testing.T) {
	fs := newFakeServer(t)
	fs.createReply = func(int32) map[string]any { return map[string]any{"errorId": 0, "taskId": 1} }
	fs.resultReply = func(n int32) map[string]any {
		if n < 3 {
			return map[string]any{"errorId": 0, "status": "processing"}
		}
		return map[string]any{"errorId": 0, "status": "ready", "solution": map[string]any{"token": "ok"}}
	}
	solver := newTestSolver(t, fs)

	result, err := solver.Solve(context.Background(), map[string]any{"type": "TurnstileTaskProxyless"}, Options{})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}
	if fs.resultCalls.Load() != 3 {
		t.Errorf("getTaskResult calls = %d, want 3", fs.resultCalls.Load())
	}
	if result.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 (polling isn't a retry)", result.Attempts)
	}
}

func TestSolveRetriesOnUnsolvable(t *testing.T) {
	fs := newFakeServer(t)
	fs.createReply = func(int32) map[string]any { return map[string]any{"errorId": 0, "taskId": 1} }
	fs.resultReply = func(int32) map[string]any {
		return map[string]any{"errorId": 1, "errorCode": "ERROR_CAPTCHA_UNSOLVABLE"}
	}
	solver := newTestSolver(t, fs)

	_, err := solver.Solve(context.Background(), map[string]any{"type": "TurnstileTaskProxyless"}, Options{Retries: 5})
	if err == nil {
		t.Fatal("Solve() expected an error, got nil")
	}
	if fs.createCalls.Load() != 5 {
		t.Errorf("createTask calls = %d, want 5 (one per attempt)", fs.createCalls.Load())
	}
}

func TestSolveStopsImmediatelyOnFatalError(t *testing.T) {
	fs := newFakeServer(t)
	fs.createReply = func(int32) map[string]any {
		return map[string]any{"errorId": 1, "errorCode": "ERROR_ZERO_BALANCE"}
	}
	fs.resultReply = func(int32) map[string]any { return map[string]any{"errorId": 0, "status": "processing"} }
	solver := newTestSolver(t, fs)

	_, err := solver.Solve(context.Background(), map[string]any{"type": "TurnstileTaskProxyless"}, Options{Retries: 5})
	if err == nil {
		t.Fatal("Solve() expected an error, got nil")
	}
	if !IsFatal(err) {
		t.Errorf("IsFatal(err) = false, want true for %v", err)
	}
	if fs.createCalls.Load() != 1 {
		t.Errorf("createTask calls = %d, want 1 (fatal errors must not retry)", fs.createCalls.Load())
	}
}

func TestSolveRetriesOverride(t *testing.T) {
	fs := newFakeServer(t)
	fs.createReply = func(int32) map[string]any { return map[string]any{"errorId": 0, "taskId": 1} }
	fs.resultReply = func(int32) map[string]any {
		return map[string]any{"errorId": 1, "errorCode": "ERROR_CAPTCHA_UNSOLVABLE"}
	}
	solver := newTestSolver(t, fs)

	_, err := solver.Solve(context.Background(), map[string]any{"type": "TurnstileTaskProxyless"}, Options{Retries: 2})
	if err == nil {
		t.Fatal("Solve() expected an error, got nil")
	}
	if fs.createCalls.Load() != 2 {
		t.Errorf("createTask calls = %d, want 2 (Retries override)", fs.createCalls.Load())
	}
}

func TestOptionsNormalizeClamps(t *testing.T) {
	o := Options{Retries: 100, Timeout: 1 * time.Hour}.normalize()
	if o.Retries != 10 {
		t.Errorf("Retries = %d, want clamped to 10", o.Retries)
	}
	if o.Timeout != 600*time.Second {
		t.Errorf("Timeout = %v, want clamped to 600s", o.Timeout)
	}

	o = Options{}.normalize()
	if o.Retries != DefaultRetries {
		t.Errorf("Retries = %d, want default %d", o.Retries, DefaultRetries)
	}
	if o.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want default %v", o.Timeout, DefaultTimeout)
	}
}
