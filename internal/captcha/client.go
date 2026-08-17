package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://api.2captcha.com"

// Client talks to the 2captcha JSON API v2 (createTask / getTaskResult / getBalance /
// reportCorrect / reportIncorrect). It never logs or returns the API key.
type Client struct {
	APIKey     string
	BaseURL    string
	SoftID     int
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{APIKey: apiKey, BaseURL: DefaultBaseURL, HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}

type createTaskRequest struct {
	ClientKey string         `json:"clientKey"`
	Task      map[string]any `json:"task"`
	SoftID    int            `json:"softId,omitempty"`
}

type createTaskResponse struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
	TaskID           int64  `json:"taskId"`
}

// CreateTask submits task (which must include a "type" field) and returns the new task ID.
func (c *Client) CreateTask(ctx context.Context, task map[string]any) (int64, error) {
	var resp createTaskResponse
	err := c.call(ctx, "createTask", createTaskRequest{ClientKey: c.APIKey, Task: task, SoftID: c.SoftID}, &resp)
	if err != nil {
		return 0, err
	}
	if resp.ErrorID != 0 {
		return 0, &APIError{Code: resp.ErrorCode, Description: resp.ErrorDescription}
	}
	return resp.TaskID, nil
}

type getTaskResultRequest struct {
	ClientKey string `json:"clientKey"`
	TaskID    int64  `json:"taskId"`
}

// TaskResult is the raw getTaskResult response. Status is "processing" or "ready".
type TaskResult struct {
	ErrorID          int            `json:"errorId"`
	ErrorCode        string         `json:"errorCode"`
	ErrorDescription string         `json:"errorDescription"`
	Status           string         `json:"status"`
	Solution         map[string]any `json:"solution"`
	Cost             string         `json:"cost"`
	IP               string         `json:"ip"`
	CreateTime       int64          `json:"createTime"`
	EndTime          int64          `json:"endTime"`
	SolveCount       int            `json:"solveCount"`
}

// GetTaskResult fetches the current status of taskID. It returns an *APIError if the API
// reports errorId != 0; a result with Status == "processing" is not an error.
func (c *Client) GetTaskResult(ctx context.Context, taskID int64) (*TaskResult, error) {
	var resp TaskResult
	if err := c.call(ctx, "getTaskResult", getTaskResultRequest{ClientKey: c.APIKey, TaskID: taskID}, &resp); err != nil {
		return nil, err
	}
	if resp.ErrorID != 0 {
		return nil, &APIError{Code: resp.ErrorCode, Description: resp.ErrorDescription}
	}
	return &resp, nil
}

type getBalanceRequest struct {
	ClientKey string `json:"clientKey"`
}

type getBalanceResponse struct {
	ErrorID          int     `json:"errorId"`
	ErrorCode        string  `json:"errorCode"`
	ErrorDescription string  `json:"errorDescription"`
	Balance          float64 `json:"balance"`
}

// GetBalance returns the account's remaining balance in USD.
func (c *Client) GetBalance(ctx context.Context) (float64, error) {
	var resp getBalanceResponse
	if err := c.call(ctx, "getBalance", getBalanceRequest{ClientKey: c.APIKey}, &resp); err != nil {
		return 0, err
	}
	if resp.ErrorID != 0 {
		return 0, &APIError{Code: resp.ErrorCode, Description: resp.ErrorDescription}
	}
	return resp.Balance, nil
}

type reportRequest struct {
	ClientKey string `json:"clientKey"`
	TaskID    int64  `json:"taskId"`
}

type reportResponse struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
}

// Report submits feedback on a solved task via reportCorrect or reportIncorrect. It feeds
// 2captcha's quality loop and can trigger refunds for bad reCAPTCHA v3 tokens.
func (c *Client) Report(ctx context.Context, taskID int64, good bool) error {
	method := "reportIncorrect"
	if good {
		method = "reportCorrect"
	}
	var resp reportResponse
	if err := c.call(ctx, method, reportRequest{ClientKey: c.APIKey, TaskID: taskID}, &resp); err != nil {
		return err
	}
	if resp.ErrorID != 0 {
		return &APIError{Code: resp.ErrorCode, Description: resp.ErrorDescription}
	}
	return nil
}

func (c *Client) call(ctx context.Context, method string, body, out any) error {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("2captcha: encode %s request: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/"+method, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("2captcha: build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("2captcha: %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("2captcha: read %s response: %w", method, err)
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("2captcha: %s: server error %d", method, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("2captcha: %s: rate limited (429)", method)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("2captcha: %s: unexpected status %d", method, resp.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("2captcha: decode %s response: %w", method, err)
	}
	return nil
}
