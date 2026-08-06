# Agent Instructions: twocaptchamcp

This repo ships an MCP server and CLI (`twocap`) that solve captchas via 2captcha.com. This file
tells any AI agent — Claude Code, Codex, Cursor, Aider, or otherwise — how to use it correctly.

The tool solves captchas; it does not detect or bypass them on its own. You (the agent) must find
the sitekey and page URL yourself — read the page HTML/DOM for `data-sitekey`, `k=`, or a widget
script src — then hand those values to the tool. Every solve costs real money on the configured
2captcha account. Don't call `captcha_solve` speculatively or wrap it in your own retry loop; the
tool already retries internally (default 5 attempts).

## Workflow

1. Identify the captcha type on the page (`g-recaptcha`, `h-captcha`, `cf-turnstile` classes or
   attributes, or a widget script src) and extract its sitekey and the page URL.
2. If unsure of the exact 2captcha type name or its required fields, call `captcha_list_types`
   (MCP) or `twocap types [family]` (CLI) — don't guess field names.
3. Call `captcha_solve` (MCP) or `twocap solve <family> URL SITEKEY` (CLI) with the type and
   fields.
4. Use the returned `token` as the value to inject into the page's hidden response field, or to
   submit with the form/API call.
5. Optionally report back with `captcha_report` / `twocap report ID --good|--bad` if the token was
   accepted or rejected downstream — this refunds cost on bad reCAPTCHA v3 tokens and improves
   future solves.

## MCP tools

- **`captcha_solve`** — `{type, task, retries?, timeout_seconds?, allow_unknown_type?}`. Blocks
  until solved or retries are exhausted (default 5 attempts, ~180s timeout each). Returns
  `{task_id, solution, token, cost, attempts, elapsed_ms}`.
- **`captcha_create_task`** — same `{type, task}` input but returns immediately with `{task_id}`;
  use when you'd rather poll yourself than block.
- **`captcha_get_result`** — `{task_id, type?}` polls a task created above; `status` is
  `"processing"` or `"ready"`.
- **`captcha_list_types`** — `{family?}` (recaptcha, turnstile, hcaptcha, funcaptcha, geetest,
  amazon, image) lists supported types with required/optional fields and solution keys.
- **`captcha_balance`** — no input; returns `{balance}` in USD.
- **`captcha_report`** — `{task_id, correct}` feeds solve-quality feedback back to 2captcha.

## CLI equivalent (`twocap`)

- `twocap balance` — account balance
- `twocap types [family]` — same catalog as `captcha_list_types`
- `twocap solve recaptcha-v2 URL SITEKEY [--invisible] [--enterprise] [--proxy scheme://user:pass@host:port]`
- `twocap solve recaptcha-v3 URL SITEKEY [--action verify] [--min-score 0.4] [--enterprise]`
- `twocap solve turnstile URL SITEKEY [--action A] [--cdata C] [--proxy ...]`
- `twocap solve hcaptcha URL SITEKEY [--invisible] [--proxy ...]`
- `twocap solve funcaptcha URL PUBLICKEY [--service-url U]`
- `twocap solve geetest URL GT [--challenge C] [--v4 --captcha-id ID]`
- `twocap solve amazon URL SITEKEY --iv IV --context CTX`
- `twocap solve image FILE|URL|- [--phrase] [--numeric N] [--comment TEXT]`
- `twocap solve text "question"`
- `twocap solve --type TYPE --task '{"field":"value"}'` — generic escape hatch for any catalog
  type, or an arbitrary type via `--allow-unknown-type`
- `twocap task create --type TYPE --task JSON` / `twocap task result ID [--wait]` — async variant
- `twocap report ID --good|--bad`

Global flags: `--json` (prefer for programmatic parsing), `--quiet`/`-q` (prints only the token,
good for `$(...)` capture), `--retries`, `--timeout`, `--verbose` (traces each attempt/poll to
stderr — use first when a solve fails unexpectedly).

Exit codes: `0` solved, `1` usage error, `2` fatal API error (bad key, zero balance — do not
retry, fix configuration or top up the account instead), `3` retries exhausted (the sitekey/type
may be wrong, or the captcha is genuinely unsolvable), `4` timeout.

## Choosing the right type

- Use the `Proxyless` variant (e.g. `RecaptchaV2TaskProxyless`) unless the solve must originate
  from a specific proxy/geo — that's the common case and what the CLI family subcommands default
  to.
- reCAPTCHA v3 and its Enterprise variant need `pageAction`/`minScore`, not a sitekey-only call —
  a wrong or default `minScore` can produce a token the target site still rejects as low-trust.
- Image/text tasks (`ImageToTextTask`, `TextCaptchaTask`) don't take a URL — they take a base64
  image body or a plain-text question, respectively.

## When a solve fails

- A fatal error (bad key, zero balance, invalid sitekey, oversized image) will not be fixed by
  retrying — surface it instead of calling `captcha_solve` again with the same input.
- `ERROR_CAPTCHA_UNSOLVABLE` after all retries usually means the sitekey, page URL, or task type
  is wrong, not that the captcha is simply hard — re-verify the extracted values before retrying.
- Never fabricate a token if solving fails — report the failure.

## Configuration

`TWOCAPTCHA_API_KEY` must be set (env var or `.envrc`/`.env`, both gitignored — never commit a
real key). See `README.md` for the full configuration table and deployment instructions.
