# certyn-cli

Go-based Certyn CLI for developer loops, CI gates, and self-hosted runner operations.

## Install

Pinned install (recommended):

```bash
curl -fsSL https://certyn.io/install | bash -s -- --version v0.1.0
```

Latest install:

```bash
curl -fsSL https://certyn.io/install | bash -s -- --latest
```

PowerShell (Windows):

```powershell
iex "& { $(iwr -useb https://certyn.io/install.ps1) } -Version v0.1.0"
```

## Build

```bash
cd certyn-cli
go build ./...
```

## Core Commands

```bash
certyn ci run smoke --project my-app --environment staging --wait
certyn ci status <run-id>
certyn ci cancel <run-id>
certyn ci list --project my-app

certyn verify --project my-app --url https://my-tunnel.ngrok-free.app
certyn verify --project my-app --url https://my-tunnel.ngrok-free.app --suite regression
certyn verify --project my-app --url https://my-tunnel.ngrok-free.app --suite checkout-suite
certyn verify --project my-app --url https://my-tunnel.ngrok-free.app --tag checkout --tag payments
certyn verify --project my-app --environment staging
certyn verify --project my-app --environment staging --suite checkout-suite

certyn ask --project my-app "Why did checkout fail after login?"
certyn ask "What should I check next?" --context "verify failed with network_401 on /api/checkout"
certyn ask --project my-app --max-tool-iterations 6 --max-output-tokens 1000 "How do we stabilize smoke?"

certyn runners pools list
certyn runners pools create --name "pool-a"
certyn runners tokens create --pool <pool-id> --mode single_use
certyn runners list
certyn runners drain <runner-id>
certyn runners resume <runner-id>

certyn projects list
certyn env list --project my-app
certyn env vars list --project my-app --env staging
certyn executions diagnose --project my-app <execution-id>
certyn executions conversation --project my-app <execution-id>

certyn config init
certyn config set --profile dev --api-url https://api.certyn.io --project my-app --environment staging --api-key-ref dev_key
certyn config use dev
certyn config show
```

`certyn verify` supports two target modes:
- Ephemeral mode: pass `--url` (user-managed public URL such as ngrok), CLI creates/deletes a temporary environment.
- Existing mode: pass `--environment`, CLI uses that environment directly and does not create/delete environments.

Use exactly one target: `--url` or `--environment`.
`--suite` supports `smoke`/`regression` aliases and custom process slugs discovered from the API.
`verify --json` includes:
- deterministic schema version (`schema_version`)
- execution-level summaries (`executions`) and totals
- failed-execution diagnostics (`diagnostics`) by default when the gate fails
- per-execution diagnostics errors (`diagnostics_errors`) without changing gate exit behavior

Use `certyn executions diagnose --project <project> <execution-id>` for compact, machine-readable failure analysis, and `certyn executions conversation --project <project> <execution-id>` for the full raw event stream.

`certyn ask` runs advisor mode only (`/api/chat/advisor`) in single-turn, non-streaming mode.
It is safe-by-default for agents and does not expose full chat tool execution behavior in v1.

## Configuration Precedence

1. CLI flags
2. Environment variables (`CERTYN_API_URL`, `CERTYN_API_KEY`, `CERTYN_PROJECT`, `CERTYN_ENVIRONMENT`, `CERTYN_PROFILE`)
3. Active config profile
4. Built-in defaults

## Process Aliases

- `smoke` -> `smoke-suite`
- `regression` -> `regression-suite`
- `explore` -> `app-explorer`
