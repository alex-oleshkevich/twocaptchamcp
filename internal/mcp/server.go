package mcp

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/captcha"
	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the 2captcha solver in an MCP server exposing solve/create/result/report/
// balance/list-types tools.
type Server struct {
	solver *captcha.Solver
	client *captcha.Client
	sdk    *sdk.Server
}

func New(client *captcha.Client, version string) *Server {
	server := &Server{solver: captcha.NewSolver(client), client: client}
	server.sdk = sdk.NewServer(&sdk.Implementation{
		Name: "twocaptchamcp", Title: "2captcha Solver", Version: version,
	}, &sdk.ServerOptions{
		Instructions: "Solve captchas via 2captcha.com. Use captcha_list_types to discover task " +
			"types and required fields, then captcha_solve with the sitekey/URL you found on the " +
			"page. captcha_solve blocks until solved or all retries are exhausted; use " +
			"captcha_create_task/captcha_get_result if you'd rather poll yourself.",
		KeepAlive: 30 * time.Second,
	})
	server.registerTools()
	return server
}

func (s *Server) Handler() http.Handler {
	handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s.sdk }, &sdk.StreamableHTTPOptions{
		SessionTimeout: 30 * time.Minute,
	})
	return http.NewCrossOriginProtection().Handler(handler)
}

func (s *Server) Run(ctx context.Context, transport sdk.Transport) error {
	return s.sdk.Run(ctx, transport)
}

func (s *Server) registerTools() {
	sdk.AddTool(s.sdk, solveTool(), func(ctx context.Context, _ *sdk.CallToolRequest, input solveInput) (*sdk.CallToolResult, solveOutput, error) {
		task, taskType, err := prepareTask(input.Type, input.Task, input.AllowUnknownType)
		if err != nil {
			return nil, solveOutput{}, err
		}
		opts := captcha.Options{Retries: input.Retries, Timeout: time.Duration(input.TimeoutSeconds) * time.Second}
		result, err := s.solver.Solve(ctx, task, opts)
		if err != nil {
			return nil, solveOutput{}, err
		}
		return nil, solveOutput{
			TaskID:    result.TaskID,
			Solution:  result.Solution,
			Token:     captcha.Token(taskType, result.Solution),
			Cost:      result.Cost,
			Attempts:  result.Attempts,
			ElapsedMS: result.ElapsedMS,
		}, nil
	})

	sdk.AddTool(s.sdk, createTaskTool(), func(ctx context.Context, _ *sdk.CallToolRequest, input createTaskInput) (*sdk.CallToolResult, createTaskOutput, error) {
		task, _, err := prepareTask(input.Type, input.Task, input.AllowUnknownType)
		if err != nil {
			return nil, createTaskOutput{}, err
		}
		taskID, err := s.client.CreateTask(ctx, task)
		if err != nil {
			return nil, createTaskOutput{}, err
		}
		return nil, createTaskOutput{TaskID: taskID}, nil
	})

	sdk.AddTool(s.sdk, getResultTool(), func(ctx context.Context, _ *sdk.CallToolRequest, input getResultInput) (*sdk.CallToolResult, getResultOutput, error) {
		res, err := s.client.GetTaskResult(ctx, input.TaskID)
		if err != nil {
			return nil, getResultOutput{}, err
		}
		out := getResultOutput{Status: res.Status, Cost: res.Cost}
		if res.Status == "ready" {
			out.Solution = res.Solution
			if taskType, ok := captcha.ByName(input.Type); ok {
				out.Token = captcha.Token(taskType, res.Solution)
			}
		}
		return nil, out, nil
	})

	sdk.AddTool(s.sdk, listTypesTool(), func(_ context.Context, _ *sdk.CallToolRequest, input listTypesInput) (*sdk.CallToolResult, listTypesOutput, error) {
		var out listTypesOutput
		for _, t := range captcha.ByFamily(input.Family) {
			out.Types = append(out.Types, taskTypeInfo{
				Name: t.Name, Family: t.Family, Description: t.Description,
				Required: t.Required, Optional: t.Optional, SolutionKeys: t.SolutionKeys,
			})
		}
		return nil, out, nil
	})

	sdk.AddTool(s.sdk, balanceTool(), func(ctx context.Context, _ *sdk.CallToolRequest, _ emptyInput) (*sdk.CallToolResult, balanceOutput, error) {
		balance, err := s.client.GetBalance(ctx)
		if err != nil {
			return nil, balanceOutput{}, err
		}
		return nil, balanceOutput{Balance: balance}, nil
	})

	sdk.AddTool(s.sdk, reportTool(), func(ctx context.Context, _ *sdk.CallToolRequest, input reportInput) (*sdk.CallToolResult, successOutput, error) {
		if err := s.client.Report(ctx, input.TaskID, input.Correct); err != nil {
			return nil, successOutput{}, err
		}
		return nil, successOutput{Success: true}, nil
	})
}

// prepareTask validates type against the catalog (unless allowUnknown), checks required fields,
// and returns the assembled 2captcha task map plus the catalog entry (zero value if unknown).
func prepareTask(taskType string, fields map[string]any, allowUnknown bool) (map[string]any, captcha.TaskType, error) {
	if taskType == "" {
		return nil, captcha.TaskType{}, fmt.Errorf("type is required; call captcha_list_types to see supported task types")
	}
	entry, known := captcha.ByName(taskType)
	if !known && !allowUnknown {
		msg := fmt.Sprintf("unknown task type %q", taskType)
		if suggestions := captcha.SuggestNames(taskType); len(suggestions) > 0 {
			msg += fmt.Sprintf("; did you mean: %s?", strings.Join(suggestions, ", "))
		}
		msg += " Call captcha_list_types for the full catalog, or set allow_unknown_type=true to bypass this check."
		return nil, captcha.TaskType{}, fmt.Errorf("%s", msg)
	}
	if known {
		if missing := captcha.MissingRequired(entry, fields); len(missing) > 0 {
			return nil, captcha.TaskType{}, fmt.Errorf("task %q is missing required field(s): %s", taskType, strings.Join(missing, ", "))
		}
	}
	task := make(map[string]any, len(fields)+1)
	maps.Copy(task, fields)
	task["type"] = taskType
	return task, entry, nil
}

func solveTool() *sdk.Tool {
	openWorld := true
	return &sdk.Tool{
		Name: "captcha_solve",
		Description: "Solve a captcha via 2captcha.com, blocking until solved or retries are exhausted. " +
			"Costs money per attempt. Use captcha_list_types to discover the type name and required task fields.",
		OutputSchema: unconstrainedOutputSchema(),
		Annotations:  &sdk.ToolAnnotations{OpenWorldHint: &openWorld},
	}
}

func createTaskTool() *sdk.Tool {
	openWorld := true
	return &sdk.Tool{
		Name:         "captcha_create_task",
		Description:  "Submit a captcha task to 2captcha without waiting for the solution. Costs money. Use captcha_get_result to poll it.",
		OutputSchema: unconstrainedOutputSchema(),
		Annotations:  &sdk.ToolAnnotations{OpenWorldHint: &openWorld},
	}
}

func getResultTool() *sdk.Tool {
	openWorld := true
	return &sdk.Tool{
		Name:         "captcha_get_result",
		Description:  "Poll the status of a task created with captcha_create_task. Returns status \"processing\" or \"ready\".",
		OutputSchema: unconstrainedOutputSchema(),
		Annotations:  &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld},
	}
}

func listTypesTool() *sdk.Tool {
	openWorld := true
	return &sdk.Tool{
		Name:         "captcha_list_types",
		Description:  "List 2captcha task types this server knows about, with required/optional fields and solution keys.",
		OutputSchema: unconstrainedOutputSchema(),
		Annotations:  &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld},
	}
}

func balanceTool() *sdk.Tool {
	openWorld := true
	return &sdk.Tool{
		Name:         "captcha_balance",
		Description:  "Get the remaining 2captcha account balance in USD.",
		OutputSchema: unconstrainedOutputSchema(),
		Annotations:  &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld},
	}
}

func reportTool() *sdk.Tool {
	openWorld := true
	destructive := false
	return &sdk.Tool{
		Name:         "captcha_report",
		Description:  "Report whether a solved task's answer was correct or not, feeding 2captcha's quality loop.",
		OutputSchema: unconstrainedOutputSchema(),
		Annotations:  &sdk.ToolAnnotations{OpenWorldHint: &openWorld, DestructiveHint: &destructive},
	}
}

func unconstrainedOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{}}
}
