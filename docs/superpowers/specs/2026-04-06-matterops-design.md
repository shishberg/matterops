# MatterOps Design Spec

## Overview

MatterOps is a single Go binary that orchestrates service deployments on a single server. It connects to Mattermost as a bot, receives GitHub webhooks, and provides a read-only web dashboard.

## Configuration

### Global config: `config.yaml`

```yaml
mattermost:
  url: "https://mattermost.example.com"
  channel: "ops"

webhook:
  port: 8080

dashboard:
  port: 8081

services_dir: "./services"  # default
```

Secrets are not stored in this file. It is safe to commit.

### Secrets: `.env`

```
MATTERMOST_TOKEN=xoxb-...
GITHUB_WEBHOOK_SECRET=whsec_...
```

Loaded at startup. Gitignored. A `.env.example` is committed as a template.

### Per-service config: `services/<name>.yaml`

The service name defaults to the filename (sans `.yaml` extension). Each file defines one service.

```yaml
branch: main
repo: "github.com/org/myapp"
working_dir: "/opt/myapp"
deploy:
  - "git pull origin main"
  - "make test"
  - "make build"

# Backend: one of the following

# Backend — exactly one of these must be present:
# Child process (first-class, not dev-only):
process:
  cmd: "go run ./cmd/myapp"
# OR system service (auto-detects systemctl on Linux, launchctl on macOS):
# service_name: myapp

# Deployment behavior
# require_confirmation: false  (default: auto-deploy on push)
```

`require_confirmation` defaults to `false` — push to the configured branch triggers an automatic deploy. Set to `true` to require a human `@matterops confirm <service>` in Mattermost before deploying.

## Architecture

Single binary, all components run as goroutines sharing a central `*service.Manager`.

### Package layout

```
matterops/
├── cmd/matterops/          # main entrypoint
├── internal/
│   ├── config/             # loads config.yaml + .env + service files
│   ├── service/            # service manager + backend abstraction
│   │   ├── manager.go      # orchestrates deploys, holds state, deploy queues
│   │   ├── systemctl.go    # systemctl backend
│   │   ├── launchctl.go    # launchctl backend
│   │   └── process.go      # child process backend
│   ├── deploy/             # runs deploy step sequences, captures output
│   ├── bot/                # Mattermost bot (websocket listener, command parser)
│   ├── webhook/            # GitHub webhook HTTP handler
│   └── dashboard/          # web dashboard (Go templates, HTTP server)
├── templates/              # HTML templates for dashboard
├── config.yaml
├── services/
├── .env.example
├── Makefile
├── go.mod
└── go.sum
```

### Component responsibilities

- **`config`**: Loads `config.yaml`, reads `.env`, discovers and parses all files in `services_dir`. Validates config at startup.
- **`service.Manager`**: Central coordinator. Holds service state, owns the deploy queues, provides methods that bot/webhook/dashboard call. Only component that mutates service state.
- **`service` backends**: Interface with three implementations (`process`, `systemctl`, `launchctl`). Each implements `Start()`, `Stop()`, `Restart()`, `Status()`. The `process` backend is first-class — not a dev-only shim.
- **`deploy`**: Executes an ordered list of shell commands for a service. Runs each step sequentially, captures combined stdout/stderr. Stops on first failure, reports which step failed and its output.
- **`bot`**: Connects to Mattermost via websocket. Watches for `@matterops` mentions in the configured channel. Parses commands and calls into `service.Manager`. Posts results/errors back to the channel.
- **`webhook`**: HTTP handler on `POST /webhook/github`. Validates `X-Hub-Signature-256`, parses push events, matches repo+branch to service configs, submits to deploy queue (or posts confirmation request if `require_confirmation` is set). Single endpoint handles all repos.
- **`dashboard`**: HTTP server with Go `html/template` rendering. Single page showing all services in a table with status, last deploy time, last result. Click to expand for deploy output. Auto-refreshes via polling.

## Service State

Each service tracks:

```go
type ServiceState struct {
    Status     string    // "running", "stopped", "deploying", "failed"
    LastDeploy time.Time
    LastResult string    // "success", "failed"
    LastOutput string    // combined output from deploy steps
    FailedStep string    // which step failed, if any
}
```

## Deploy Flow

1. Deploy request enters the service's queue
2. Worker picks it up, sets status to "deploying", posts to Mattermost: "Deploying myapp..."
3. Runs each deploy step sequentially, capturing output
4. If a step fails: status -> "failed", post error + failed step + output to Mattermost, stop
5. If all steps pass: restart the service via the backend, status -> "running", post success to Mattermost

### Deploy Queue

Each service has a channel-based deploy queue with capacity 1 (latest wins):

- New deploy request replaces any pending request in the queue (only the latest matters)
- Currently running deploy is never interrupted
- Worker goroutine per service drains the queue
- On completion, checks for a pending deploy and runs it if present
- Mattermost feedback: "Deploy queued for myapp (deploy already in progress)"

### Confirmation Flow

When `require_confirmation: true`:

1. Webhook receives push, matches to service
2. Bot posts: "Push to main on critical-api. Deploy? Reply `@matterops confirm critical-api`"
3. Bot watches for the confirm message, submits to deploy queue
4. Confirmations expire after a configurable timeout (default 10 minutes)

## Bot Commands

- `@matterops status` — list all services and their current state
- `@matterops deploy <service>` — manually trigger a deploy
- `@matterops restart <service>` — restart without redeploying
- `@matterops confirm <service>` — confirm a pending deploy

## GitHub Webhook

- Endpoint: `POST /webhook/github`
- Validates `X-Hub-Signature-256` against secret from `GITHUB_WEBHOOK_SECRET` env var
- Parses push events, extracts repo and ref (branch)
- Matches against service configs by `repo` + `branch`
- Non-push events are ignored
- Unmatched repos/branches return 200 (no error, just no action)

## Web Dashboard

- Server-rendered HTML via Go `html/template`
- Read-only status page
- Table: service name, status (color-coded), last deploy time, last result
- Expandable rows showing last deploy output
- Auto-refresh via polling
- No authentication (use reverse proxy with basic auth if needed)
- Served on its own port

## Dev Environment & Agentic Loop

### Makefile

```makefile
check: lint test build        ## Run all checks

lint:                         ## Run linter
	golangci-lint run ./...

test:                         ## Run tests
	go test ./...

build:                        ## Build binary
	go build -o bin/matterops ./cmd/matterops

dev:                          ## Run in dev mode
	go run ./cmd/matterops --config config.dev.yaml

playwright:                   ## Run Playwright dashboard tests
	npx playwright test
```

`make check` is the single command for CI and agentic development loops.

### Dev configuration

A `config.dev.yaml` and `services/` directory with dummy services using the `process` backend, so `make dev` works out of the box. Same code paths as production — the process backend is a first-class backend, not a dev shim.

### Testing strategy

- **Unit tests**: config parsing, deploy queue logic, webhook signature validation, bot command parsing
- **Integration tests**: spin up the binary in dev mode with mock services (simple HTTP servers or sleep processes), verify deploy flows end-to-end
- **Playwright tests**: dashboard renders correctly, shows service states, updates after deploys
