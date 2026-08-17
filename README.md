# twocaptchamcp

An MCP server (and CLI) that solves captchas via [2captcha.com](https://2captcha.com/2captcha-api):
reCAPTCHA v2/v3, Cloudflare Turnstile, hCaptcha, FunCaptcha, GeeTest, Amazon WAF, and image/text
captchas.

## Installation

The latest `twocap` release is available on [GitHub Releases](https://github.com/alex-oleshkevich/twocaptchamcp/releases/latest). Install it with the [GitHub CLI](https://cli.github.com/):

```sh
repo=alex-oleshkevich/twocaptchamcp

case "$(uname -s):$(uname -m)" in
  Linux:x86_64) asset='twocaptchamcp_*_linux_amd64.tar.gz' ;;
  Linux:aarch64|Linux:arm64) asset='twocaptchamcp_*_linux_arm64.tar.gz' ;;
  Darwin:x86_64) asset='twocaptchamcp_*_darwin_amd64.tar.gz' ;;
  Darwin:arm64) asset='twocaptchamcp_*_darwin_arm64.tar.gz' ;;
  *) printf 'Unsupported platform: %s\n' "$(uname -s):$(uname -m)" >&2; exit 1 ;;
esac

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
gh release download --repo "$repo" --pattern "$asset" --dir "$tmpdir"
tar -xzf "$tmpdir"/*.tar.gz -C "$tmpdir"
mkdir -p "$HOME/.local/bin"
install -m 755 "$tmpdir/twocap" "$HOME/.local/bin/twocap"
```

The command downloads the matching archive from the latest release. Add `$HOME/.local/bin` to
your `PATH` if it is not already there, then set the required `TWOCAPTCHA_API_KEY` described below.
Windows users can download the matching `.zip` archive from the [latest release](https://github.com/alex-oleshkevich/twocaptchamcp/releases/latest).

## Development

Run the local checks and test suite with [just](https://just.systems/):

```sh
just check
just test
```

Create a release from a clean `main` branch with `just release patch`, `just release minor`, or
`just release major`. The recipe builds a changelog from commits since the previous release,
creates an annotated semantic-version tag, and asks for confirmation before committing and
atomically pushing the branch and tag to `origin`.

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

`compose.yaml` publishes the MCP port via uncloud's built-in ingress (`TWOCAP_DOMAIN:8080/https`),
which handles TLS termination automatically. twocap itself still enforces `TWOCAPTCHAMCP_TOKEN`
as a bearer token on `/mcp` — callers must send `Authorization: Bearer <token>`.
