# twocaptchamcp

An MCP server and CLI for solving captchas through [2captcha.com](https://2captcha.com/2captcha-api):
reCAPTCHA v2/v3, Cloudflare Turnstile, hCaptcha, FunCaptcha, GeeTest, Amazon WAF, and image/text
captchas.

## Installation

Download the latest `twocap` release from [GitHub Releases](https://github.com/alex-oleshkevich/twocaptchamcp/releases/latest). On Linux or macOS, use `curl`:

```sh
repo=alex-oleshkevich/twocaptchamcp

case "$(uname -s):$(uname -m)" in
  Linux:x86_64) asset_suffix='_linux_amd64.tar.gz' ;;
  Linux:aarch64|Linux:arm64) asset_suffix='_linux_arm64.tar.gz' ;;
  Darwin:x86_64) asset_suffix='_darwin_amd64.tar.gz' ;;
  Darwin:arm64) asset_suffix='_darwin_arm64.tar.gz' ;;
  *) printf 'Unsupported platform: %s\n' "$(uname -s):$(uname -m)" >&2; exit 1 ;;
esac

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
curl -fsSL "https://api.github.com/repos/$repo/releases/latest" -o "$tmpdir/release.json"
download_url=$(awk -v suffix="$asset_suffix" '
  /"browser_download_url":/ && index($0, suffix) {
    sub(/^.*"browser_download_url": "/, "")
    sub(/".*$/, "")
    print
    exit
  }
' "$tmpdir/release.json")
if [ -z "$download_url" ]; then
  printf 'No release archive found for this platform.\n' >&2
  exit 1
fi
curl -fsSL "$download_url" -o "$tmpdir/release.tar.gz"
tar -xzf "$tmpdir/release.tar.gz" -C "$tmpdir"
mkdir -p "$HOME/.local/bin"
install -m 755 "$tmpdir/twocap" "$HOME/.local/bin/twocap"
```

This downloads the archive for your platform from the latest release. It requires `curl`, `awk`,
`tar`, and `install`, which are standard on Linux and macOS. Add `$HOME/.local/bin` to your `PATH`
if needed, then set `TWOCAPTCHA_API_KEY` as described below.

To install manually, download the archive for your operating system and architecture from the
[latest release](https://github.com/alex-oleshkevich/twocaptchamcp/releases/latest), extract it,
and put the `twocap` binary in a directory on your `PATH`. Windows users can download the matching
`.zip` archive and add its extracted directory to `PATH`.

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
