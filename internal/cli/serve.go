package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/config"
	"github.com/alex-oleshkevich/twocaptchamcp/internal/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	urfave "github.com/urfave/cli/v3"
)

func (a *app) serveCommand() *urfave.Command {
	return &urfave.Command{
		Name:  "mcp",
		Usage: "Serve the MCP server over streamable HTTP",
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			cfg, err := config.FromEnvironment()
			if err != nil {
				return usageError(err.Error())
			}
			a.client.BaseURL = cfg.BaseURL
			a.client.SoftID = cfg.SoftID
			server := mcp.New(a.client, cmd.Root().Version)

			mux := http.NewServeMux()
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			mux.Handle("/mcp", server.Handler())

			if _, err := fmt.Fprintf(a.stderr, "twocap mcp listening on %s (unauthenticated)\n", cfg.Address); err != nil {
				return err
			}
			return http.ListenAndServe(cfg.Address, mux)
		},
	}
}

func (a *app) stdioCommand() *urfave.Command {
	return &urfave.Command{
		Name:  "stdio",
		Usage: "Serve the MCP server over stdio",
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			server := mcp.New(a.client, cmd.Root().Version)
			return server.Run(ctx, &sdk.StdioTransport{})
		},
	}
}
