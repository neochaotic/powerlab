.PHONY: dev build test check lint lint-go clean help stage-build test-backend test-ui test-ui-watch

# ─── Frontend ──────────────────────────────────────────────────────────

dev: ## Start UI dev server
	cd ui && npm run dev

build: ## Build static SPA
	cd ui && npm run build

test: test-ui ## Run all tests

test-ui: ## Run frontend unit tests
	cd ui && npx vitest run

test-ui-watch: ## Run frontend tests in watch mode
	cd ui && npx vitest

check: ## TypeScript type checking
	cd ui && npm run check

lint: ## Lint frontend
	cd ui && npx eslint .

lint-go: ## Lint Go backend per service (warn-only, matches CI, ADR-0040)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Install via: brew install golangci-lint OR go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	@for svc in common core gateway app-management user-service local-storage message-bus; do \
		echo "── $$svc ──"; \
		(cd backend/$$svc && golangci-lint run --config=../../.golangci.yml --timeout=5m ./...) || true; \
	done

# ─── Backend ───────────────────────────────────────────────────────────

test-backend: ## Run Go backend tests
	cd backend/core && go test ./... -v
	cd backend/gateway && go test ./... -v
	cd backend/app-management && go test ./... -v
	cd backend/user-service && go test ./... -v
	cd backend/local-storage && go test ./... -v
	cd backend/message-bus && go test ./... -v

# ─── Staging deploy build (Linux/amd64 from any host) ────────────────

stage-build: ## Build hot-swap binaries for Linux/amd64 (CGO svcs from release, #414)
	bash scripts/stage-build.sh

# ─── Utilities ─────────────────────────────────────────────────────────

clean: ## Clean build artifacts
	rm -rf ui/build ui/.svelte-kit ui/node_modules/.vite

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
