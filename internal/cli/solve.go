package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/captcha"
	urfave "github.com/urfave/cli/v3"
)

func (a *app) solveCommand(retries, timeoutSeconds *int) *urfave.Command {
	var taskType string
	var taskRaw string

	generic := &urfave.Command{
		Name:  "solve",
		Usage: "Solve a captcha (generic escape hatch or a family subcommand)",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "type", Usage: "2captcha task type, e.g. TurnstileTaskProxyless", Destination: &taskType},
			&urfave.StringFlag{Name: "task", Usage: "task JSON: inline, @file.json, or @- for stdin", Destination: &taskRaw},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			if taskType == "" {
				return usageError("--type is required")
			}
			fields, err := readTaskJSON(taskRaw)
			if err != nil {
				return usageError(err.Error())
			}
			return a.runSolve(ctx, taskType, fields, *retries, *timeoutSeconds)
		},
		Commands: familySolveCommands(a, retries, timeoutSeconds),
	}
	return generic
}

func readTaskJSON(raw string) (map[string]any, error) {
	var data []byte
	switch {
	case raw == "":
		return map[string]any{}, nil
	case raw == "@-":
		var err error
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
	case strings.HasPrefix(raw, "@"):
		var err error
		data, err = os.ReadFile(raw[1:])
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", raw[1:], err)
		}
	default:
		data = []byte(raw)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("parse --task JSON: %w", err)
	}
	return fields, nil
}

// runSolve validates taskType/fields against the catalog, runs Solve, and renders the result
// according to the app's --json/--quiet/--verbose flags.
func (a *app) runSolve(ctx context.Context, taskType string, fields map[string]any, retries, timeoutSeconds int) error {
	entry, known := captcha.ByName(taskType)
	if !known {
		msg := fmt.Sprintf("unknown task type %q", taskType)
		if suggestions := captcha.SuggestNames(taskType); len(suggestions) > 0 {
			msg += fmt.Sprintf("; did you mean: %s? (use --type to override)", strings.Join(suggestions, ", "))
		}
		return usageError(msg)
	}
	if missing := captcha.MissingRequired(entry, fields); len(missing) > 0 {
		return usageError(fmt.Sprintf("task %q is missing required field(s): %s", taskType, strings.Join(missing, ", ")))
	}

	task := make(map[string]any, len(fields)+1)
	maps.Copy(task, fields)
	task["type"] = taskType

	if a.verbose {
		fmt.Fprintf(a.stderr, "solving %s (retries=%d timeout=%ds)\n", taskType, retries, timeoutSeconds)
	}

	result, err := a.solver.Solve(ctx, task, captcha.Options{Retries: retries, Timeout: time.Duration(timeoutSeconds) * time.Second})
	if err != nil {
		return err
	}
	token := captcha.Token(entry, result.Solution)

	if a.quiet {
		fmt.Fprintln(a.stdout, token)
		return nil
	}
	if a.jsonOutput {
		return a.printJSON(map[string]any{
			"task_id": result.TaskID, "solution": result.Solution, "token": token,
			"cost": result.Cost, "attempts": result.Attempts, "elapsed_ms": result.ElapsedMS,
		})
	}
	fmt.Fprintf(a.stdout, "token:    %s\n", token)
	fmt.Fprintf(a.stdout, "task_id:  %d\n", result.TaskID)
	fmt.Fprintf(a.stdout, "cost:     %s\n", result.Cost)
	fmt.Fprintf(a.stdout, "attempts: %d\n", result.Attempts)
	fmt.Fprintf(a.stdout, "elapsed:  %dms\n", result.ElapsedMS)
	return nil
}

// proxyFields parses --proxy scheme://[user:pass@]host:port into 2captcha's proxy* fields and
// reports whether a proxied task type should be used instead of the *Proxyless variant.
func proxyFields(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return nil, fmt.Errorf("--proxy must look like scheme://[user:pass@]host:port")
	}
	userinfo, hostport := "", rest
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		userinfo, hostport = rest[:at], rest[at+1:]
	}
	host, portStr, err := splitHostPort(hostport)
	if err != nil {
		return nil, fmt.Errorf("--proxy: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("--proxy: invalid port %q", portStr)
	}
	fields := map[string]any{"proxyType": scheme, "proxyAddress": host, "proxyPort": port}
	if userinfo != "" {
		login, password, _ := strings.Cut(userinfo, ":")
		fields["proxyLogin"] = login
		fields["proxyPassword"] = password
	}
	return fields, nil
}

func splitHostPort(hostport string) (string, string, error) {
	idx := strings.LastIndex(hostport, ":")
	if idx < 0 {
		return "", "", fmt.Errorf("missing port in %q", hostport)
	}
	return hostport[:idx], hostport[idx+1:], nil
}

func familySolveCommands(a *app, retries, timeoutSeconds *int) []*urfave.Command {
	return []*urfave.Command{
		recaptchaV2Command(a, retries, timeoutSeconds),
		recaptchaV3Command(a, retries, timeoutSeconds),
		turnstileCommand(a, retries, timeoutSeconds),
		hcaptchaCommand(a, retries, timeoutSeconds),
		funcaptchaCommand(a, retries, timeoutSeconds),
		geetestCommand(a, retries, timeoutSeconds),
		amazonCommand(a, retries, timeoutSeconds),
		imageCommand(a, retries, timeoutSeconds),
		textCommand(a, retries, timeoutSeconds),
	}
}

func recaptchaV2Command(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var invisible, enterprise bool
	var dataS, proxy string
	return &urfave.Command{
		Name: "recaptcha-v2", Usage: "Solve Google reCAPTCHA v2: recaptcha-v2 URL SITEKEY",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: "invisible", Destination: &invisible},
			&urfave.BoolFlag{Name: "enterprise", Destination: &enterprise},
			&urfave.StringFlag{Name: "data-s", Destination: &dataS, Usage: "Enterprise data-s payload"},
			&urfave.StringFlag{Name: "proxy", Destination: &proxy, Usage: "scheme://[user:pass@]host:port"},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			url, key, err := twoArgs(cmd, "URL", "SITEKEY")
			if err != nil {
				return err
			}
			fields := map[string]any{"websiteURL": url, "websiteKey": key}
			if invisible {
				fields["isInvisible"] = true
			}
			if dataS != "" {
				fields["dataS"] = dataS
			}
			taskType := "RecaptchaV2TaskProxyless"
			if enterprise {
				taskType = "RecaptchaV2EnterpriseTaskProxyless"
			}
			taskType, fields, err = applyProxy(taskType, fields, proxy)
			if err != nil {
				return usageError(err.Error())
			}
			return a.runSolve(ctx, taskType, fields, *retries, *timeoutSeconds)
		},
	}
}

func recaptchaV3Command(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var action string
	var minScore float64
	var enterprise bool
	return &urfave.Command{
		Name: "recaptcha-v3", Usage: "Solve Google reCAPTCHA v3: recaptcha-v3 URL SITEKEY",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "action", Value: "verify", Destination: &action},
			&urfave.Float64Flag{Name: "min-score", Value: 0.4, Destination: &minScore},
			&urfave.BoolFlag{Name: "enterprise", Destination: &enterprise},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			url, key, err := twoArgs(cmd, "URL", "SITEKEY")
			if err != nil {
				return err
			}
			fields := map[string]any{"websiteURL": url, "websiteKey": key, "pageAction": action, "minScore": minScore}
			taskType := "RecaptchaV3TaskProxyless"
			if enterprise {
				taskType = "RecaptchaV3EnterpriseTaskProxyless"
			}
			return a.runSolve(ctx, taskType, fields, *retries, *timeoutSeconds)
		},
	}
}

func turnstileCommand(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var action, cdata, proxy string
	return &urfave.Command{
		Name: "turnstile", Usage: "Solve Cloudflare Turnstile: turnstile URL SITEKEY",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "action", Destination: &action},
			&urfave.StringFlag{Name: "cdata", Destination: &cdata},
			&urfave.StringFlag{Name: "proxy", Destination: &proxy, Usage: "scheme://[user:pass@]host:port"},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			url, key, err := twoArgs(cmd, "URL", "SITEKEY")
			if err != nil {
				return err
			}
			fields := map[string]any{"websiteURL": url, "websiteKey": key}
			if action != "" {
				fields["action"] = action
			}
			if cdata != "" {
				fields["cdata"] = cdata
			}
			taskType, fields, err := applyProxy("TurnstileTaskProxyless", fields, proxy)
			if err != nil {
				return usageError(err.Error())
			}
			return a.runSolve(ctx, taskType, fields, *retries, *timeoutSeconds)
		},
	}
}

func hcaptchaCommand(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var invisible bool
	var proxy string
	return &urfave.Command{
		Name: "hcaptcha", Usage: "Solve hCaptcha: hcaptcha URL SITEKEY",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: "invisible", Destination: &invisible},
			&urfave.StringFlag{Name: "proxy", Destination: &proxy, Usage: "scheme://[user:pass@]host:port"},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			url, key, err := twoArgs(cmd, "URL", "SITEKEY")
			if err != nil {
				return err
			}
			fields := map[string]any{"websiteURL": url, "websiteKey": key}
			if invisible {
				fields["isInvisible"] = true
			}
			taskType, fields, err := applyProxy("HCaptchaTaskProxyless", fields, proxy)
			if err != nil {
				return usageError(err.Error())
			}
			return a.runSolve(ctx, taskType, fields, *retries, *timeoutSeconds)
		},
	}
}

func funcaptchaCommand(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var serviceURL string
	return &urfave.Command{
		Name: "funcaptcha", Usage: "Solve Arkose Labs FunCaptcha: funcaptcha URL PUBLICKEY",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "service-url", Destination: &serviceURL, Usage: "funcaptchaApiJSSubdomain"},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			url, key, err := twoArgs(cmd, "URL", "PUBLICKEY")
			if err != nil {
				return err
			}
			fields := map[string]any{"websiteURL": url, "websitePublicKey": key}
			if serviceURL != "" {
				fields["funcaptchaApiJSSubdomain"] = serviceURL
			}
			return a.runSolve(ctx, "FunCaptchaTaskProxyless", fields, *retries, *timeoutSeconds)
		},
	}
}

func geetestCommand(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var challenge, apiServer string
	var v4 bool
	var captchaID string
	return &urfave.Command{
		Name: "geetest", Usage: "Solve GeeTest v3/v4: geetest URL GT",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "challenge", Destination: &challenge},
			&urfave.StringFlag{Name: "api-server", Destination: &apiServer, Usage: "geetestApiServerSubdomain"},
			&urfave.BoolFlag{Name: "v4", Destination: &v4},
			&urfave.StringFlag{Name: "captcha-id", Destination: &captchaID},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			url, gt, err := twoArgs(cmd, "URL", "GT")
			if err != nil {
				return err
			}
			fields := map[string]any{"websiteURL": url, "gt": gt}
			if challenge != "" {
				fields["challenge"] = challenge
			}
			if apiServer != "" {
				fields["geetestApiServerSubdomain"] = apiServer
			}
			if v4 {
				fields["version"] = 4
				fields["captchaId"] = captchaID
			}
			return a.runSolve(ctx, "GeeTestTaskProxyless", fields, *retries, *timeoutSeconds)
		},
	}
}

func amazonCommand(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var iv, captchaContext string
	return &urfave.Command{
		Name: "amazon", Usage: "Solve Amazon WAF captcha: amazon URL SITEKEY --iv IV --context CTX",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "iv", Destination: &iv, Required: true},
			&urfave.StringFlag{Name: "context", Destination: &captchaContext, Required: true},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			url, key, err := twoArgs(cmd, "URL", "SITEKEY")
			if err != nil {
				return err
			}
			fields := map[string]any{"websiteURL": url, "websiteKey": key, "iv": iv, "context": captchaContext}
			return a.runSolve(ctx, "AmazonTaskProxyless", fields, *retries, *timeoutSeconds)
		},
	}
}

func imageCommand(a *app, retries, timeoutSeconds *int) *urfave.Command {
	var phrase, caseSensitive bool
	var numeric int
	var comment string
	return &urfave.Command{
		Name: "image", Usage: "Solve an image captcha: image FILE|URL|-",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: "phrase", Destination: &phrase},
			&urfave.BoolFlag{Name: "case-sensitive", Destination: &caseSensitive},
			&urfave.IntFlag{Name: "numeric", Destination: &numeric, Usage: "1=only digits, 2=no digits"},
			&urfave.StringFlag{Name: "comment", Destination: &comment},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			if cmd.Args().Len() != 1 {
				return usageError("expected exactly 1 argument: FILE|URL|-")
			}
			body, err := loadImage(ctx, cmd.Args().Get(0))
			if err != nil {
				return usageError(err.Error())
			}
			fields := map[string]any{"body": body}
			if phrase {
				fields["phrase"] = true
			}
			if caseSensitive {
				fields["case"] = true
			}
			if numeric != 0 {
				fields["numeric"] = numeric
			}
			if comment != "" {
				fields["comment"] = comment
			}
			return a.runSolve(ctx, "ImageToTextTask", fields, *retries, *timeoutSeconds)
		},
	}
}

const maxImageBytes = 100 << 10 // 2captcha rejects images over 100kB with ERROR_TOO_BIG_CAPTCHA_FILESIZE

func loadImage(ctx context.Context, source string) (string, error) {
	var data []byte
	switch {
	case source == "-":
		var err error
		data, err = io.ReadAll(io.LimitReader(os.Stdin, maxImageBytes+1))
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("fetch %s: %w", source, err)
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
		if err != nil {
			return "", err
		}
	default:
		var err error
		data, err = os.ReadFile(source)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", source, err)
		}
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty image (2captcha rejects this as ERROR_ZERO_CAPTCHA_FILESIZE)")
	}
	if len(data) > maxImageBytes {
		return "", fmt.Errorf("image is over 100kB (2captcha rejects this as ERROR_TOO_BIG_CAPTCHA_FILESIZE)")
	}
	return base64.StdEncoding.EncodeToString(bytes.TrimSpace(data)), nil
}

func textCommand(a *app, retries, timeoutSeconds *int) *urfave.Command {
	return &urfave.Command{
		Name: "text", Usage: "Solve a text question/answer captcha: text \"question\"",
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			if cmd.Args().Len() != 1 {
				return usageError("expected exactly 1 argument: the question text")
			}
			return a.runSolve(ctx, "TextCaptchaTask", map[string]any{"comment": cmd.Args().Get(0)}, *retries, *timeoutSeconds)
		},
	}
}

func twoArgs(cmd *urfave.Command, name1, name2 string) (string, string, error) {
	if cmd.Args().Len() != 2 {
		return "", "", usageError(fmt.Sprintf("expected exactly 2 arguments: %s %s", name1, name2))
	}
	return cmd.Args().Get(0), cmd.Args().Get(1), nil
}

// applyProxy merges proxy fields (if raw is non-empty) into fields and switches proxylessType
// (e.g. "TurnstileTaskProxyless") to its proxied counterpart (e.g. "TurnstileTask").
func applyProxy(proxylessType string, fields map[string]any, raw string) (string, map[string]any, error) {
	if raw == "" {
		return proxylessType, fields, nil
	}
	proxy, err := proxyFields(raw)
	if err != nil {
		return "", nil, err
	}
	maps.Copy(fields, proxy)
	return strings.TrimSuffix(proxylessType, "Proxyless"), fields, nil
}
