# twocaptchamcp

An MCP server and CLI for solving captchas through [2captcha.com](https://2captcha.com/2captcha-api):
reCAPTCHA v2/v3, Cloudflare Turnstile, hCaptcha, FunCaptcha, GeeTest, Amazon WAF, and image/text
captchas.

## Installation

Download the archive for your operating system and architecture from the [latest release](https://github.com/alex-oleshkevich/twocaptchamcp/releases/latest).

On Linux or macOS, download the `.tar.gz` archive, then extract and install it:

```sh
tar -xzf twocaptchamcp_*.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 twocap "$HOME/.local/bin/twocap"
```

If you prefer `curl`, copy the asset URL from the release page and download it before running the
same commands:

```sh
curl -fLO '<release-asset-url>'
```

Windows users can download the matching `.zip` archive, extract it, and add its directory to `PATH`.
Set `TWOCAPTCHA_API_KEY` as described below.

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
twocap stdio   # stdio transport, for local Claude Code use
```

To register it with Claude Code:

```sh
claude mcp add twocaptcha -- twocap stdio
# or, against a running HTTP server:
claude mcp add --transport http twocaptcha http://127.0.0.1:8080/mcp
```

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
