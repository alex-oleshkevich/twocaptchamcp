package mcp

type emptyInput struct{}

type solveInput struct {
	Type             string         `json:"type" jsonschema:"2captcha task type, e.g. RecaptchaV2TaskProxyless, TurnstileTaskProxyless, HCaptchaTaskProxyless, ImageToTextTask. Call captcha_list_types for the full catalog."`
	Task             map[string]any `json:"task" jsonschema:"2captcha task fields for the given type, excluding 'type' itself (e.g. websiteURL, websiteKey)."`
	Retries          int            `json:"retries,omitempty" jsonschema:"Number of full submit+poll attempts before giving up. Default 5, max 10."`
	TimeoutSeconds   int            `json:"timeout_seconds,omitempty" jsonschema:"Seconds to poll a single attempt before it counts as failed. Default 180."`
	AllowUnknownType bool           `json:"allow_unknown_type,omitempty" jsonschema:"Set true to submit a task type not in this server's catalog."`
}

type solveOutput struct {
	TaskID    int64          `json:"task_id"`
	Solution  map[string]any `json:"solution"`
	Token     string         `json:"token,omitempty"`
	Cost      string         `json:"cost,omitempty"`
	Attempts  int            `json:"attempts"`
	ElapsedMS int64          `json:"elapsed_ms"`
}

type createTaskInput struct {
	Type             string         `json:"type" jsonschema:"2captcha task type, e.g. RecaptchaV2TaskProxyless."`
	Task             map[string]any `json:"task" jsonschema:"2captcha task fields for the given type, excluding 'type' itself."`
	AllowUnknownType bool           `json:"allow_unknown_type,omitempty" jsonschema:"Set true to submit a task type not in this server's catalog."`
}

type createTaskOutput struct {
	TaskID int64 `json:"task_id"`
}

type getResultInput struct {
	TaskID int64  `json:"task_id"`
	Type   string `json:"type,omitempty" jsonschema:"Task type originally submitted, used to extract a convenience 'token' from the solution."`
}

type getResultOutput struct {
	Status   string         `json:"status"`
	Solution map[string]any `json:"solution,omitempty"`
	Token    string         `json:"token,omitempty"`
	Cost     string         `json:"cost,omitempty"`
}

type listTypesInput struct {
	Family string `json:"family,omitempty" jsonschema:"Filter by family: recaptcha, turnstile, hcaptcha, funcaptcha, geetest, amazon, image."`
}

type taskTypeInfo struct {
	Name         string   `json:"name"`
	Family       string   `json:"family"`
	Description  string   `json:"description"`
	Required     []string `json:"required,omitempty"`
	Optional     []string `json:"optional,omitempty"`
	SolutionKeys []string `json:"solution_keys,omitempty"`
}

type listTypesOutput struct {
	Types []taskTypeInfo `json:"types"`
}

type balanceOutput struct {
	Balance float64 `json:"balance"`
}

type reportInput struct {
	TaskID  int64 `json:"task_id"`
	Correct bool  `json:"correct" jsonschema:"true if the solution worked, false if it was rejected/incorrect."`
}

type successOutput struct {
	Success bool `json:"success"`
}
