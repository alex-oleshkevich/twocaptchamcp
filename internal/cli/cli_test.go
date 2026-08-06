package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func noSleep(context.Context, time.Duration) error { return nil }

// newFakeAPI simulates the 2captcha JSON API v2 for CLI tests: createTask always succeeds and
// getTaskResult is immediately ready, unless overridden per test via the returned handler funcs.
func newFakeAPI(t *testing.T, createTaskID int64, solution map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "taskId": createTaskID})
		case "/getTaskResult":
			json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "status": "ready", "solution": solution, "cost": "0.001"})
		case "/getBalance":
			json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "balance": 3.21})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runCLI(t *testing.T, baseURL string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	testSleep = noSleep
	t.Cleanup(func() { testSleep = nil })

	var outBuf, errBuf bytes.Buffer
	full := append([]string{"--api-key", "test-key"}, args...)
	if baseURL != "" {
		full = append([]string{"--base-url", baseURL}, full...)
	}
	code = Run(context.Background(), full, &outBuf, &errBuf, "test")
	return outBuf.String(), errBuf.String(), code
}

func TestSolveRecaptchaV2FlagMapping(t *testing.T) {
	srv := newFakeAPI(t, 1, map[string]any{"gRecaptchaResponse": "resp-token"})
	stdout, stderr, code := runCLI(t, srv.URL, "solve", "recaptcha-v2", "https://example.com", "sitekey123", "--json")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitOK, stderr)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, stdout)
	}
	if out["token"] != "resp-token" {
		t.Errorf("token = %v, want resp-token", out["token"])
	}
}

func TestSolveWithProxyFlipsTaskType(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/createTask" {
			json.NewDecoder(r.Body).Decode(&gotBody)
			json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "taskId": 1})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "status": "ready", "solution": map[string]any{"token": "t"}})
	}))
	defer srv.Close()

	_, stderr, code := runCLI(t, srv.URL, "solve", "turnstile", "https://example.com", "sitekey", "--proxy", "http://user:pass@1.2.3.4:8080", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitOK, stderr)
	}
	task, _ := gotBody["task"].(map[string]any)
	if task["type"] != "TurnstileTask" {
		t.Errorf("type = %v, want TurnstileTask (proxied variant)", task["type"])
	}
	if task["proxyAddress"] != "1.2.3.4" || task["proxyLogin"] != "user" || task["proxyPassword"] != "pass" {
		t.Errorf("proxy fields not populated: %+v", task)
	}
}

func TestSolveQuietPrintsOnlyToken(t *testing.T) {
	srv := newFakeAPI(t, 1, map[string]any{"gRecaptchaResponse": "quiet-token"})
	stdout, stderr, code := runCLI(t, srv.URL, "solve", "recaptcha-v2", "https://example.com", "sitekey", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, ExitOK, stderr)
	}
	if strings.TrimSpace(stdout) != "quiet-token" {
		t.Errorf("stdout = %q, want exactly the token", stdout)
	}
}

func TestSolveGenericTypeFlag(t *testing.T) {
	srv := newFakeAPI(t, 1, map[string]any{"text": "42"})
	stdout, _, code := runCLI(t, srv.URL, "solve", "--type", "TextCaptchaTask", "--task", `{"comment":"what is 6*7?"}`, "--quiet")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	if strings.TrimSpace(stdout) != "42" {
		t.Errorf("stdout = %q, want 42", stdout)
	}
}

func TestSolveUnknownTypeExitsUsage(t *testing.T) {
	_, stderr, code := runCLI(t, "", "solve", "--type", "NotAType", "--task", "{}")
	if code != ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage(%d); stderr=%s", code, ExitUsage, stderr)
	}
}

func TestSolveMissingAPIKeyExitsUsage(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	code := Run(context.Background(), []string{"balance"}, &outBuf, &errBuf, "test")
	if code != ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage(%d); stderr=%s", code, ExitUsage, errBuf.String())
	}
}

func TestSolveFatalAPIErrorExitsFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"errorId": 1, "errorCode": "ERROR_ZERO_BALANCE"})
	}))
	defer srv.Close()

	_, stderr, code := runCLI(t, srv.URL, "solve", "--type", "TextCaptchaTask", "--task", `{"comment":"x"}`)
	if code != ExitFatalAPI {
		t.Errorf("exit code = %d, want ExitFatalAPI(%d); stderr=%s", code, ExitFatalAPI, stderr)
	}
}

func TestSolveExhaustedRetriesExitsExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "taskId": 1})
		case "/getTaskResult":
			json.NewEncoder(w).Encode(map[string]any{"errorId": 1, "errorCode": "ERROR_CAPTCHA_UNSOLVABLE"})
		}
	}))
	defer srv.Close()

	_, stderr, code := runCLI(t, srv.URL, "--retries", "2", "solve", "--type", "TextCaptchaTask", "--task", `{"comment":"x"}`)
	if code != ExitExhausted {
		t.Errorf("exit code = %d, want ExitExhausted(%d); stderr=%s", code, ExitExhausted, stderr)
	}
}

func TestBalanceJSON(t *testing.T) {
	srv := newFakeAPI(t, 1, nil)
	stdout, stderr, code := runCLI(t, srv.URL, "balance", "--json")
	if code != ExitOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if out["balance"] != 3.21 {
		t.Errorf("balance = %v, want 3.21", out["balance"])
	}
}

func TestTypesFiltersByFamily(t *testing.T) {
	stdout, stderr, code := runCLI(t, "", "types", "image")
	if code != ExitOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr)
	}
	if strings.Contains(stdout, "RecaptchaV2") {
		t.Errorf("expected no recaptcha entries when filtering by image family:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ImageToTextTask") {
		t.Errorf("expected ImageToTextTask in image family output:\n%s", stdout)
	}
}

func TestTaskCreateAndResult(t *testing.T) {
	srv := newFakeAPI(t, 123, map[string]any{"text": "created-answer"})
	stdout, stderr, code := runCLI(t, srv.URL, "task", "create", "--type", "TextCaptchaTask", "--task", `{"comment":"q"}`)
	if code != ExitOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "123" {
		t.Errorf("stdout = %q, want 123", stdout)
	}

	stdout, stderr, code = runCLI(t, srv.URL, "task", "result", "123", "--type", "TextCaptchaTask")
	if code != ExitOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "created-answer") {
		t.Errorf("expected token in output:\n%s", stdout)
	}
}

func TestReportRequiresGoodOrBad(t *testing.T) {
	_, stderr, code := runCLI(t, "", "report", "1")
	if code != ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage(%d); stderr=%s", code, ExitUsage, stderr)
	}
}
