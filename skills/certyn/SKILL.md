---
name: certyn
description: Run QA tests, verify deployments, and diagnose failures using the Certyn CLI. Triggers when users mention testing, QA, verification, CI gates, or certyn.
---

# Certyn CLI

Certyn is a QA automation platform. The `certyn` CLI lets you trigger test runs, verify deployments, and diagnose failures from the terminal.

## Prerequisites

- `certyn` must be installed and on PATH
- Authenticated via `certyn config init` or `CERTYN_API_KEY` env var
- A project must exist on Certyn (use `--project <slug>` or set default via config)

## Core Commands

### Verify a deployment (most common)

```bash
# Against a public URL (ephemeral environment — created and deleted automatically)
certyn verify --project <slug> --url <public-url>

# Against an existing environment
certyn verify --project <slug> --environment <env-name>

# Run a specific suite
certyn verify --project <slug> --environment staging --suite smoke
certyn verify --project <slug> --url <url> --suite regression
certyn verify --project <slug> --url <url> --suite <custom-process-slug>

# Filter by tags
certyn verify --project <slug> --url <url> --tag checkout --tag payments

# JSON output (for CI parsing)
certyn verify --project <slug> --environment staging --json
```

`--url` and `--environment` are mutually exclusive. Use exactly one.

Suite aliases: `smoke` → `smoke-suite`, `regression` → `regression-suite`, `explore` → `app-explorer`.

### CI commands

```bash
certyn ci run smoke --project <slug> --environment staging --wait
certyn ci status <run-id>
certyn ci cancel <run-id>
certyn ci list --project <slug>
```

### Diagnose failures

```bash
certyn executions diagnose --project <slug> <execution-id>
certyn executions conversation --project <slug> <execution-id>
```

### Ask the AI advisor

```bash
certyn ask --project <slug> "Why did checkout fail after login?"
certyn ask --project <slug> "What should I check next?" --context "verify failed with network_401 on /api/checkout"
```

### Manage environments and projects

```bash
certyn projects list
certyn env list --project <slug>
certyn env vars list --project <slug> --env staging
```

### Self-hosted runners

```bash
certyn runners pools list
certyn runners pools create --name "pool-a"
certyn runners tokens create --pool <pool-id> --mode single_use
certyn runners list
certyn runners drain <runner-id>
certyn runners resume <runner-id>
```

### Configuration

```bash
certyn config init
certyn config set --profile dev --api-url https://api.certyn.io --project my-app --environment staging --api-key-ref dev_key
certyn config use dev
certyn config show
```

## Config Precedence

1. CLI flags → 2. Environment variables (`CERTYN_API_URL`, `CERTYN_API_KEY`, `CERTYN_PROJECT`, `CERTYN_ENVIRONMENT`) → 3. Active profile → 4. Defaults

## Typical Workflows

**Pre-push verification:**
```bash
certyn verify --project my-app --url https://my-tunnel.ngrok-free.app --suite smoke
```

**CI gate (GitHub Actions):**
```bash
certyn verify --project my-app --environment staging --suite regression --json
```

**Investigate a failure:**
```bash
certyn executions diagnose --project my-app <execution-id>
```

## Exit Codes

- `0` — all tests passed
- `1` — failures detected or error
