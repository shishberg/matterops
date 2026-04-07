# MatterOps

## Implementation Rules

- **Strict TDD**: Write failing tests first, then implement the minimum code to make them pass, then refactor. No production code without a failing test driving it.
- **Commit and push after every step**: Each implementation step gets its own commit with a meaningful message describing what was added/changed and why. Always push immediately after committing — do not wait to be asked.
- **Simplicity over cleverness**: Prefer simple, readable code. Use well-established Go packages and patterns (e.g. `net/http`, `gopkg.in/yaml.v3`, `godotenv`, `testify`) over rolling your own. If a standard library or popular package solves the problem, use it. No NIH syndrome.

## Project Structure

- Single Go binary: `main.go` at project root (for `go install` support)
- Internal packages: `internal/{config,service,deploy,bot,webhook,dashboard}`
- Config: `config.yaml` (global) + `services/*.yaml` (per-service) + `.env` (secrets)
- `make check` runs lint + test + build

## Dev Environment

- `make dev` runs the bot locally with `config.dev.yaml`
- Use the `process` backend for local development (child processes, not systemctl/launchctl)
- `make check` must pass before considering any step complete
- Playwright tests for the web dashboard via `make playwright`
