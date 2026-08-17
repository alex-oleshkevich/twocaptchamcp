# twocaptchamcp

An MCP server and CLI for solving captchas through [2captcha.com](https://2captcha.com/2captcha-api):
reCAPTCHA v2/v3, Cloudflare Turnstile, hCaptcha, FunCaptcha, GeeTest, Amazon WAF, and image/text
captchas.

## Installation

Install the latest release on Linux or macOS with:

```sh
curl -fsSL https://raw.githubusercontent.com/alex-oleshkevich/twocaptchamcp/master/install.sh | bash
```

The installer detects your platform and architecture, verifies the release checksum, installs
`twocap` to `$HOME/.local/bin`, and configures shell completion. Start a new shell after
installation, then set `TWOCAPTCHA_API_KEY` as described below.

For manual installation, download the matching archive from the [latest release](https://github.com/alex-oleshkevich/twocaptchamcp/releases/latest), extract it, and put `twocap` on your `PATH`.


## Configuration

| Env | Default | Notes |
|---|---|---|
| `TWOCAPTCHA_API_KEY` | — | required |
| `TWOCAPTCHAMCP_ADDRESS` | `127.0.0.1:8080` | `mcp` command listen address |
| `TWOCAPTCHAMCP_TOKEN` | — | required for a non-loopback address; bearer-protects `/mcp` |
| `TWOCAPTCHA_BASE_URL` | `https://api.2captcha.com` | override for testing |
| `TWOCAPTCHA_SOFT_ID` | — | 2captcha developer ID |
| `TWOCAPTCHAMCP_MAX_RETRIES` | `5` | default `retries` for `captcha_solve` |
| `TWOCAPTCHAMCP_TIMEOUT` | `180s` | default per-attempt poll budget |

## CLI

```sh
export TWOCAPTCHA_API_KEY=...

twocap balance
twocap types recaptcha
twocap solve recaptcha-v2 https://example.com/page 6Lf...sitekey
twocap solve turnstile https://example.com/page 0x...sitekey --quiet
twocap solve --type ImageToTextTask --task '{"body":"<base64>"}'
twocap task create --type TurnstileTaskProxyless --task @task.json
twocap task result 123456 --wait
```

Global flags: `--json`, `--quiet`/`-q`, `--retries`/`-r`, `--timeout`/`-t`, `--verbose`/`-v`.

Exit codes: `0` solved, `1` usage/config error, `2` fatal 2captcha error (bad key, zero balance),
`3` retries exhausted, `4` timeout.

## MCP server

```sh
twocap mcp     # streamable HTTP on $TWOCAPTCHAMCP_ADDRESS, /mcp + /healthz
twocap stdio   # stdio transport for local MCP clients
```

Local stdio is the recommended setup for Claude Code, Claude Desktop, and Cursor because the
client starts `twocap` directly and no MCP HTTP port is exposed.

To register it with Claude Code:

```sh
claude mcp add --scope user --transport stdio \
  --env "TWOCAPTCHA_API_KEY=$TWOCAPTCHA_API_KEY" \
  twocap -- twocap stdio
```

For Claude Desktop, copy [`examples/mcp-stdio.json`](examples/mcp-stdio.json) into
`claude_desktop_config.json`. For Cursor, copy it into `.cursor/mcp.json` in a project or
`~/.cursor/mcp.json` for a user-wide setup. Replace the placeholder API key and restart the client;
the config starts the installed `twocap` binary with the stdio transport.

Tools: `captcha_solve`, `captcha_create_task`, `captcha_get_result`, `captcha_list_types`,
`captcha_balance`, `captcha_report`.

## Deploying

```sh
just compose-config
just compose-up
just uncloud-deploy   # uc deploy --file compose.yaml --yes
```

The `compose.yaml` file publishes the MCP port through uncloud's built-in ingress
(`TWOCAP_DOMAIN:8080/https`), which handles TLS termination. twocap still requires
`TWOCAPTCHAMCP_TOKEN` as a bearer token on `/mcp`. Callers must send
`Authorization: Bearer <token>`.

## License

This project is licensed under the [MIT License](LICENSE).

## Development

Use [just](https://just.systems/) to run the local checks and test suite:

```sh
just check
just test
```

To create a release, start from a clean `main` branch and run `just release patch`,
`just release minor`, or `just release major`. The recipe builds a changelog from commits since
the previous release, creates an annotated semantic-version tag, and asks for confirmation before
it commits and atomically pushes the branch and tag to `origin`.
