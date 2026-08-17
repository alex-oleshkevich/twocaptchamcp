// Package cli implements the twocap command-line interface: the same solve/create/result/report
// flows as the MCP tools, callable by hand for debugging or one-off captcha solving.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/captcha"
	urfave "github.com/urfave/cli/v3"
)

// Exit codes, chosen so shell scripts can branch without parsing stderr.
const (
	ExitOK        = 0
	ExitUsage     = 1
	ExitFatalAPI  = 2
	ExitExhausted = 3
	ExitTimeout   = 4
)

// testSleep overrides the solver's clock in tests to avoid real polling delays. Left nil in
// production builds.
var testSleep func(context.Context, time.Duration) error

type app struct {
	stdout io.Writer
	stderr io.Writer

	client *captcha.Client
	solver *captcha.Solver

	jsonOutput bool
	quiet      bool
	verbose    bool
}

// Run builds and executes the twocap urfave/cli command tree, returning a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, version string) int {
	a := &app{stdout: stdout, stderr: stderr}

	var apiKey, baseURL string
	var retries int
	var timeoutSeconds int

	root := &urfave.Command{
		Name:                  "twocap",
		Usage:                 "Solve captchas via 2captcha.com, by hand or as an MCP server",
		Version:               version,
		Writer:                stdout,
		ErrWriter:             stderr,
		EnableShellCompletion: true,
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "api-key", Sources: urfave.EnvVars("TWOCAPTCHA_API_KEY"), Usage: "2captcha API key", Destination: &apiKey},
			&urfave.StringFlag{Name: "base-url", Sources: urfave.EnvVars("TWOCAPTCHA_BASE_URL"), Value: captcha.DefaultBaseURL, Usage: "2captcha API base URL", Destination: &baseURL},
			&urfave.IntFlag{Name: "retries", Aliases: []string{"r"}, Sources: urfave.EnvVars("TWOCAPTCHAMCP_MAX_RETRIES"), Value: captcha.DefaultRetries, Usage: "submit+poll attempts before giving up", Destination: &retries},
			&urfave.IntFlag{Name: "timeout", Aliases: []string{"t"}, Sources: urfave.EnvVars("TWOCAPTCHAMCP_TIMEOUT_SECONDS"), Value: int(captcha.DefaultTimeout / time.Second), Usage: "seconds to poll a single attempt", Destination: &timeoutSeconds},
			&urfave.BoolFlag{Name: "json", Usage: "machine-readable JSON output", Destination: &a.jsonOutput},
			&urfave.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "print only the token/answer", Destination: &a.quiet},
			&urfave.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "log each attempt and error to stderr", Destination: &a.verbose},
		},
		Before: func(_ context.Context, cmd *urfave.Command) (context.Context, error) {
			completionRequested := cmd.Args().First() == "completion"
			for _, arg := range args {
				if arg == "--generate-shell-completion" {
					completionRequested = true
					break
				}
			}
			if apiKey == "" && cmd.Args().First() != "doctor" && !completionRequested {
				return nil, usageError("--api-key or TWOCAPTCHA_API_KEY is required")
			}
			if apiKey != "" {
				a.client = captcha.NewClient(apiKey)
				a.client.BaseURL = baseURL
				a.solver = captcha.NewSolver(a.client)
				if testSleep != nil {
					a.solver.Sleep = testSleep
				} else if a.verbose {
					a.solver.Sleep = verboseSleep(stderr)
				}
			}
			return nil, nil
		},
		Commands: []*urfave.Command{
			a.serveCommand(),
			a.stdioCommand(),
			a.solveCommand(&retries, &timeoutSeconds),
			a.taskCommand(),
			a.reportCommand(),
			a.balanceCommand(),
			a.typesCommand(),
			a.doctorCommand(&apiKey, &baseURL),
		},
	}

	if err := root.Run(ctx, append([]string{"twocap"}, args...)); err != nil {
		return a.reportError(err)
	}
	return ExitOK
}

// usageError marks err as a CLI usage problem (ExitUsage), distinct from an API/solve failure.
type usageErr struct{ msg string }

func (e *usageErr) Error() string { return e.msg }

func usageError(msg string) error { return &usageErr{msg: msg} }

func (a *app) reportError(err error) int {
	_, _ = fmt.Fprintln(a.stderr, "error:", err)
	var ue *usageErr
	switch {
	case errors.As(err, &ue):
		return ExitUsage
	case captcha.IsFatal(err):
		return ExitFatalAPI
	case errors.Is(err, context.DeadlineExceeded):
		return ExitTimeout
	default:
		return ExitExhausted
	}
}

// verboseSleep wraps the real clock with a progress line printed before each wait, so --verbose
// traces exactly the retry/poll timeline a production failure would have taken.
func verboseSleep(stderr io.Writer) func(context.Context, time.Duration) error {
	return func(ctx context.Context, d time.Duration) error {
		_, _ = fmt.Fprintf(stderr, "  waiting %s...\n", d.Round(time.Second))
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
}

// printJSON writes v as indented JSON to a.stdout.
func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
