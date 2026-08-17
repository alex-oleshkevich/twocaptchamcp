package cli

import (
	"context"
	"fmt"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/config"
	urfave "github.com/urfave/cli/v3"
)

type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// doctorCommand runs a battery of environment/connectivity checks and never itself requires an
// API key up front — unlike every other command — because reporting "no API key configured" is
// exactly the kind of thing a doctor command exists to surface.
func (a *app) doctorCommand(apiKey, baseURL *string) *urfave.Command {
	return &urfave.Command{
		Name:  "doctor",
		Usage: "Check configuration and connectivity to 2captcha",
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			checks := a.runDoctorChecks(ctx, *apiKey, *baseURL)

			if a.jsonOutput {
				return a.printJSON(checks)
			}
			failed := 0
			for _, c := range checks {
				mark := "ok  "
				if !c.OK {
					mark = "FAIL"
					failed++
				}
				if _, err := fmt.Fprintf(a.stdout, "[%s] %-20s %s\n", mark, c.Name, c.Detail); err != nil {
					return err
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d check(s) failed", failed)
			}
			return nil
		},
	}
}

func (a *app) runDoctorChecks(ctx context.Context, apiKey, baseURL string) []doctorCheck {
	var checks []doctorCheck

	if apiKey == "" {
		checks = append(checks, doctorCheck{"api-key", false, "not set (--api-key or TWOCAPTCHA_API_KEY)"})
	} else if len(apiKey) != 32 {
		checks = append(checks, doctorCheck{"api-key", true, fmt.Sprintf("set (%d chars — 2captcha keys are usually 32)", len(apiKey))})
	} else {
		checks = append(checks, doctorCheck{"api-key", true, "set"})
	}

	checks = append(checks, doctorCheck{"base-url", true, baseURL})

	if apiKey == "" {
		checks = append(checks, doctorCheck{"connectivity", false, "skipped — no API key to test with"})
	} else {
		balance, err := a.client.GetBalance(ctx)
		if err != nil {
			checks = append(checks, doctorCheck{"connectivity", false, err.Error()})
		} else {
			checks = append(checks, doctorCheck{"connectivity", true, fmt.Sprintf("connected, balance $%.4f", balance)})
		}
	}

	if cfg, err := config.FromEnvironment(); err != nil {
		checks = append(checks, doctorCheck{"mcp-serve-config", false, err.Error()})
	} else {
		checks = append(checks, doctorCheck{"mcp-serve-config", true, fmt.Sprintf("address=%s token-set=%t", cfg.Address, cfg.Token != "")})
	}

	return checks
}
