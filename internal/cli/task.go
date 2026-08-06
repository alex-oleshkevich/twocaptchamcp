package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/captcha"
	urfave "github.com/urfave/cli/v3"
)

func (a *app) taskCommand() *urfave.Command {
	return &urfave.Command{
		Name:  "task",
		Usage: "Create or poll a 2captcha task without the blocking solve loop",
		Commands: []*urfave.Command{
			a.taskCreateCommand(),
			a.taskResultCommand(),
		},
	}
}

func (a *app) taskCreateCommand() *urfave.Command {
	var taskType, taskRaw string
	return &urfave.Command{
		Name:  "create",
		Usage: "Submit a task and print its ID without waiting for the solution",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "type", Destination: &taskType, Required: true},
			&urfave.StringFlag{Name: "task", Destination: &taskRaw, Usage: "task JSON: inline, @file.json, or @- for stdin"},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			fields, err := readTaskJSON(taskRaw)
			if err != nil {
				return usageError(err.Error())
			}
			fields["type"] = taskType
			taskID, err := a.client.CreateTask(ctx, fields)
			if err != nil {
				return err
			}
			if a.jsonOutput {
				return a.printJSON(map[string]any{"task_id": taskID})
			}
			fmt.Fprintln(a.stdout, taskID)
			return nil
		},
	}
}

func (a *app) taskResultCommand() *urfave.Command {
	var wait bool
	var taskType string
	return &urfave.Command{
		Name:      "result",
		Usage:     "Poll a task's status: task result ID [--wait]",
		ArgsUsage: "ID",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: "wait", Destination: &wait, Usage: "block until status is ready"},
			&urfave.StringFlag{Name: "type", Destination: &taskType, Usage: "task type, used to extract a convenience token"},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			if cmd.Args().Len() != 1 {
				return usageError("expected exactly 1 argument: ID")
			}
			taskID, err := strconv.ParseInt(cmd.Args().Get(0), 10, 64)
			if err != nil {
				return usageError("ID must be an integer")
			}
			for {
				res, err := a.client.GetTaskResult(ctx, taskID)
				if err != nil {
					return err
				}
				if res.Status == "ready" || !wait {
					token := ""
					if entry, ok := captcha.ByName(taskType); ok {
						token = captcha.Token(entry, res.Solution)
					}
					if a.jsonOutput {
						return a.printJSON(map[string]any{"status": res.Status, "solution": res.Solution, "token": token, "cost": res.Cost})
					}
					fmt.Fprintf(a.stdout, "status: %s\n", res.Status)
					if res.Status == "ready" {
						fmt.Fprintf(a.stdout, "token:  %s\n", token)
						fmt.Fprintf(a.stdout, "cost:   %s\n", res.Cost)
					}
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
			}
		},
	}
}

func (a *app) reportCommand() *urfave.Command {
	var good, bad bool
	return &urfave.Command{
		Name:      "report",
		Usage:     "Report a solved task as correct or incorrect: report ID --good|--bad",
		ArgsUsage: "ID",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: "good", Destination: &good},
			&urfave.BoolFlag{Name: "bad", Destination: &bad},
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			if cmd.Args().Len() != 1 {
				return usageError("expected exactly 1 argument: ID")
			}
			if good == bad {
				return usageError("exactly one of --good or --bad is required")
			}
			taskID, err := strconv.ParseInt(cmd.Args().Get(0), 10, 64)
			if err != nil {
				return usageError("ID must be an integer")
			}
			if err := a.client.Report(ctx, taskID, good); err != nil {
				return err
			}
			fmt.Fprintln(a.stdout, "ok")
			return nil
		},
	}
}
