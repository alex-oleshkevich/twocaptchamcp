# twocaptchamcp

An MCP server (and CLI) that solves captchas via [2captcha.com](https://2captcha.com/2captcha-api):
reCAPTCHA v2/v3, Cloudflare Turnstile, hCaptcha, FunCaptcha, GeeTest, Amazon WAF, and image/text
captchas.

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

Register with Claude Code:

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

`compose.yaml` runs `twocap mcp` behind a Caddy proxy that basic-auths external callers and
injects the bearer token twocap itself checks. Generate the password hash with:

```sh
just caddy-hash-password 'your-password'
```
