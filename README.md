# MatterOps

A single-binary service orchestration bot for a single server. Manages deployments via Mattermost commands and GitHub webhooks, with a read-only web dashboard.

## Features

- **Mattermost bot** -- listens for `@matterops` commands to deploy, restart, and check service status
- **GitHub webhooks** -- auto-deploys on push to a configured branch (per-service opt-in confirmation)
- **Web dashboard** -- read-only status page showing all services, deploy history, and output
- **Three service backends** -- child processes, systemctl (Linux), launchctl (macOS)
- **Per-service deploy queue** -- latest-wins, no redundant deploys

## Quick Start

### Prerequisites

- Go 1.22+
- A Mattermost server with a bot account
- (Optional) golangci-lint for linting

### Build

```bash
make build    # produces bin/matterops
```

### Configure

1. Copy and edit the global config:

```yaml
# config.yaml
mattermost:
  url: "https://mattermost.example.com"
  channel: "ops"

webhook:
  port: 8080

dashboard:
  port: 8081

services_dir: "./services"
```

2. Create a `.env` file with secrets (see `.env.example`):

```
MATTERMOST_TOKEN=your-bot-token
GITHUB_WEBHOOK_SECRET=your-webhook-secret
```

3. Add service configs in `services/`. One YAML file per service -- the filename becomes the service name:

```yaml
# services/myapp.yaml
branch: main
repo: "org/myapp"
working_dir: "/opt/myapp"
deploy:
  - "git pull origin main"
  - "make test"
  - "make build"

# Backend -- pick one:
process:
  cmd: "./bin/myapp serve"
# OR for a systemd/launchd service:
# service_name: myapp
```

### Run

```bash
./bin/matterops --config config.yaml --env .env
```

The bot connects to Mattermost, the webhook server starts on the configured port, and the dashboard is available at `http://localhost:8081`.

## Bot Commands

In the configured Mattermost channel:

| Command | Description |
|---|---|
| `@matterops status` | List all services and their current state |
| `@matterops deploy <service>` | Manually trigger a deploy |
| `@matterops restart <service>` | Restart a service without redeploying |
| `@matterops confirm <service>` | Confirm a pending deploy (for services with `require_confirmation: true`) |

## GitHub Webhooks

Point your repo's webhook at `http://your-server:8080/webhook/github` with content type `application/json` and the secret from your `.env`.

On push to a service's configured branch:
- **Default:** auto-deploys immediately
- **With `require_confirmation: true`:** posts to Mattermost and waits for `@matterops confirm <service>`

## Service Configuration

Each file in `services/` defines one service:

| Field | Required | Description |
|---|---|---|
| `branch` | yes | Branch to watch for deploys |
| `repo` | yes | GitHub repo (`org/repo` format) |
| `working_dir` | yes | Directory to run deploy steps in |
| `deploy` | yes | Ordered list of shell commands to run |
| `process.cmd` | one backend required | Command to run as a child process |
| `service_name` | one backend required | systemctl/launchctl unit name |
| `require_confirmation` | no | Default `false`. If `true`, require Mattermost confirmation before deploying |

## Deploy Flow

1. Deploy steps run sequentially in the service's `working_dir`
2. If any step fails, the deploy stops and the error is posted to Mattermost
3. If all steps pass, the service is restarted via its backend
4. A per-service queue (capacity 1, latest wins) prevents redundant deploys

## Development

```bash
make dev          # run with config.dev.yaml (uses process backend)
make check        # lint + test + build
make test         # tests only
make playwright   # Playwright dashboard tests (requires server running)
```

`make check` is the single command for CI.

## License

See [LICENSE](LICENSE).
