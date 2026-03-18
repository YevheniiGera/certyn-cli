# certyn-cli

`certyn` is the Certyn CLI for agent guidance, validation runs, and failure triage.

It now has two auth paths:

- Humans: `certyn login`
- CI/agents: `CERTYN_API_KEY` or `certyn config set --api-key-ref ...`

## Install

Pinned install:

```bash
curl -fsSL https://certyn.io/install | bash -s -- --version v0.1.0
```

Latest install:

```bash
curl -fsSL https://certyn.io/install | bash -s -- --latest
```

PowerShell:

```powershell
iex "& { $(iwr -useb https://certyn.io/install.ps1) } -Version v0.1.0"
```

## First Run

Interactive local setup:

```bash
certyn init
certyn login
certyn doctor
certyn whoami
```

Automation setup:

```bash
certyn config set --profile ci --api-url https://api.certyn.io --project my-app --environment staging --api-key-ref ci_key
certyn config use ci
```

## Core Workflow

```bash
certyn ask "What should I check before changing the login flow?"

certyn login
certyn whoami
certyn doctor

certyn run smoke --project my-app --url https://my-tunnel.ngrok-free.app --wait
certyn run smoke --project my-app --environment staging --wait
certyn run --project my-app --environment staging --tag login --tag checkout --wait

certyn run status <run-id>
certyn run cancel <run-id>

certyn diagnose --project my-app <execution-id>
certyn config show

certyn update
certyn uninstall
```

## Browser Auth

Browser auth uses Auth0 device flow.

The CLI resolves these settings in this order:

1. CLI flags
2. Environment variables
3. Profile config
4. Built-in defaults

Supported auth environment variables:

- `CERTYN_AUTH_ISSUER`
- `CERTYN_AUTH_AUDIENCE`
- `CERTYN_AUTH_CLIENT_ID`

Defaults:

- issuer: `https://auth.certyn.io`
- client ID: built-in `Certyn CLI` Auth0 native app
- audience: inferred from `CERTYN_API_URL`

Example:

```bash
certyn login

certyn login --profile dev --api-url https://dev.api.certyn.io

certyn login --profile local --api-url https://local.api.certyn.io
```

## Automation Auth

CI and other non-interactive flows should continue using API keys:

```bash
export CERTYN_API_KEY=...
certyn run smoke --project my-app --environment staging --wait
```

Or store the key locally:

```bash
certyn config set --profile ci --api-key-ref ci_key --api-key "$CERTYN_API_KEY"
```

## Advanced Commands

These remain available, but they are no longer the primary on-ramp:

- `issues`
- `observations`
- `projects`
- `environments`
- `runners`
- `tests`
- `executions`

Use `certyn --help` and `certyn <command> --help` for details.

## Development

```bash
go build ./...
go test ./...
```
