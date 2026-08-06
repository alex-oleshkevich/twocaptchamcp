package cli

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

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

			var handler http.Handler = mux
			if cfg.Token != "" {
				handler = bearerToken(cfg.Token, mux)
			}

			fmt.Fprintf(a.stderr, "twocap mcp listening on %s\n", cfg.Address)
			return http.ListenAndServe(cfg.Address, handler)
		},
	}
}

// bearerToken guards the /mcp path with a constant-time comparison against token. Health checks
// stay open so container orchestration doesn't need credentials. Defense in depth even when a
// reverse proxy (Caddy) also authenticates.
func bearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "valid bearer token required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
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
