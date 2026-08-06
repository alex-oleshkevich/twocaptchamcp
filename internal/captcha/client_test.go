package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientCreateTaskSendsClientKey(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "taskId": 7})
	}))
	defer srv.Close()

	client := &Client{APIKey: "secret-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
	taskID, err := client.CreateTask(context.Background(), map[string]any{"type": "TextCaptchaTask", "comment": "2+2?"})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if taskID != 7 {
		t.Errorf("taskID = %d, want 7", taskID)
	}
	if gotBody["clientKey"] != "secret-key" {
		t.Errorf("clientKey = %v, want secret-key", gotBody["clientKey"])
	}
}

func TestClientCreateTaskReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"errorId": 1, "errorCode": "ERROR_ZERO_BALANCE", "errorDescription": "no funds"})
	}))
	defer srv.Close()

	client := &Client{APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := client.CreateTask(context.Background(), map[string]any{"type": "TextCaptchaTask"})
	if err == nil {
		t.Fatal("CreateTask() expected an error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.Code != "ERROR_ZERO_BALANCE" {
		t.Errorf("Code = %s, want ERROR_ZERO_BALANCE", apiErr.Code)
	}
}

func TestClientErrorNeverLeaksAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"errorId": 1, "errorCode": "ERROR_KEY_DOES_NOT_EXIST", "errorDescription": "bad key"})
	}))
	defer srv.Close()

	const secret = "super-secret-api-key-value"
	client := &Client{APIKey: secret, BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := client.GetBalance(context.Background())
	if err == nil {
		t.Fatal("GetBalance() expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error message leaked the API key: %v", err)
	}
}

func TestClientGetTaskResultProcessing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "status": "processing"})
	}))
	defer srv.Close()

	client := &Client{APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	res, err := client.GetTaskResult(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTaskResult() error = %v", err)
	}
	if res.Status != "processing" {
		t.Errorf("Status = %s, want processing", res.Status)
	}
}

func TestClientGetBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "balance": 12.34})
	}))
	defer srv.Close()

	client := &Client{APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	balance, err := client.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBalance() error = %v", err)
	}
	if balance != 12.34 {
		t.Errorf("balance = %v, want 12.34", balance)
	}
}

func TestClientServerErrorIsNotFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := &Client{APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := client.GetBalance(context.Background())
	if err == nil {
		t.Fatal("expected an error for HTTP 500")
	}
	if IsFatal(err) {
		t.Error("HTTP transport error must not be classified fatal")
	}
}
