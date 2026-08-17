package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/captcha"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// newTestClient wires an in-memory MCP client/server pair around a Server backed by a fake
// 2captcha HTTP server, and returns a connected ClientSession for the test to drive.
func newTestClient(t *testing.T, createReply, resultReply map[string]any) *sdk.ClientSession {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			_ = json.NewEncoder(w).Encode(createReply)
		case "/getTaskResult":
			_ = json.NewEncoder(w).Encode(resultReply)
		case "/getBalance":
			_ = json.NewEncoder(w).Encode(map[string]any{"errorId": 0, "balance": 5.5})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.Close)

	client := &captcha.Client{APIKey: "test", BaseURL: fake.URL, HTTPClient: fake.Client()}
	server := New(client, "test")

	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.sdk.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server Connect() error = %v", err)
	}
	mcpClient := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestListToolsIncludesAllSixTools(t *testing.T) {
	session := newTestClient(t, nil, nil)
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	want := map[string]bool{
		"captcha_solve": false, "captcha_create_task": false, "captcha_get_result": false,
		"captcha_list_types": false, "captcha_balance": false, "captcha_report": false,
	}
	for _, tool := range result.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not found in tools/list", name)
		}
	}
}

func TestCaptchaSolveUnknownTypeReturnsHelpfulError(t *testing.T) {
	session := newTestClient(t, nil, nil)
	args, _ := json.Marshal(map[string]any{"type": "NotARealType", "task": map[string]any{}})
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "captcha_solve", Arguments: json.RawMessage(args)})
	if err != nil {
		t.Fatalf("CallTool() protocol error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for an unknown task type")
	}
}

func TestCaptchaSolveMissingRequiredFieldReturnsError(t *testing.T) {
	session := newTestClient(t, nil, nil)
	args, _ := json.Marshal(map[string]any{"type": "RecaptchaV2TaskProxyless", "task": map[string]any{"websiteURL": "https://example.com"}})
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "captcha_solve", Arguments: json.RawMessage(args)})
	if err != nil {
		t.Fatalf("CallTool() protocol error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for a missing required field (websiteKey)")
	}
}

func TestCaptchaSolveSuccess(t *testing.T) {
	session := newTestClient(t,
		map[string]any{"errorId": 0, "taskId": 99},
		map[string]any{"errorId": 0, "status": "ready", "solution": map[string]any{"token": "solved-token"}, "cost": "0.002"},
	)
	args, _ := json.Marshal(map[string]any{
		"type": "TurnstileTaskProxyless",
		"task": map[string]any{"websiteURL": "https://example.com", "websiteKey": "0xkey"},
	})
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "captcha_solve", Arguments: json.RawMessage(args)})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %+v", result.Content)
	}
	out, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent = %T, want map[string]any", result.StructuredContent)
	}
	if out["token"] != "solved-token" {
		t.Errorf("token = %v, want solved-token", out["token"])
	}
}

func TestCaptchaBalance(t *testing.T) {
	session := newTestClient(t, nil, nil)
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "captcha_balance"})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %+v", result.Content)
	}
	out := result.StructuredContent.(map[string]any)
	if out["balance"] != 5.5 {
		t.Errorf("balance = %v, want 5.5", out["balance"])
	}
}

func TestCaptchaListTypesFiltersByFamily(t *testing.T) {
	session := newTestClient(t, nil, nil)
	args, _ := json.Marshal(map[string]any{"family": "turnstile"})
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "captcha_list_types", Arguments: json.RawMessage(args)})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	out := result.StructuredContent.(map[string]any)
	types, ok := out["types"].([]any)
	if !ok || len(types) == 0 {
		t.Fatalf("types = %v, want a non-empty list", out["types"])
	}
	for _, raw := range types {
		entry := raw.(map[string]any)
		if entry["family"] != "turnstile" {
			t.Errorf("got family %v in turnstile filter", entry["family"])
		}
	}
}
