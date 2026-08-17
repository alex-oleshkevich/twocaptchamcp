package cli

import (
	"context"
	"fmt"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/captcha"
	urfave "github.com/urfave/cli/v3"
)

func (a *app) balanceCommand() *urfave.Command {
	return &urfave.Command{
		Name:  "balance",
		Usage: "Print the remaining 2captcha account balance",
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			balance, err := a.client.GetBalance(ctx)
			if err != nil {
				return err
			}
			if a.jsonOutput {
				return a.printJSON(map[string]any{"balance": balance})
			}
			_, err = fmt.Fprintf(a.stdout, "%.4f\n", balance)
			return err
		},
	}
}

func (a *app) typesCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "types",
		Usage:     "List known 2captcha task types, optionally filtered by family",
		ArgsUsage: "[family]",
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			family := ""
			if cmd.Args().Len() > 0 {
				family = cmd.Args().Get(0)
			}
			entries := captcha.ByFamily(family)
			if a.jsonOutput {
				return a.printJSON(entries)
			}
			for _, t := range entries {
				if _, err := fmt.Fprintf(a.stdout, "%-36s %-10s %s\n", t.Name, t.Family, t.Description); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(a.stdout, "  required: %v\n", t.Required); err != nil {
					return err
				}
				if len(t.Optional) > 0 {
					if _, err := fmt.Fprintf(a.stdout, "  optional: %v\n", t.Optional); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}
