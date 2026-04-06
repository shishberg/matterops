.PHONY: check lint test build dev playwright clean

check: lint test build ## Run all checks

lint: ## Run linter
	golangci-lint run ./...

test: ## Run tests
	go test -race -count=1 ./...

build: ## Build binary
	go build -o bin/matterops ./cmd/matterops

dev: ## Run in dev mode
	go run ./cmd/matterops --config config.dev.yaml

playwright: ## Run Playwright dashboard tests
	npx playwright test

clean: ## Clean build artifacts
	rm -rf bin/
