package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

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

			listener, err := net.Listen("tcp", cfg.Address)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", cfg.Address, err)
			}
			return serveHTTP(ctx, listener, mux, a.stderr)
		},
	}
}

func serveHTTP(ctx context.Context, listener net.Listener, handler http.Handler, stderr io.Writer) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if _, err := fmt.Fprintf(stderr, "twocap mcp listening on %s\n", listener.Addr()); err != nil {
		_ = listener.Close()
		return err
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-done:
		}
	}()

	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
		return nil
	}
	return err
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
