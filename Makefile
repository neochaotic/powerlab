.PHONY: dev build test check lint clean help

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

# ─── Backend ───────────────────────────────────────────────────────────

test-backend: ## Run Go backend tests
	cd backend/core && go test ./... -v
	cd backend/gateway && go test ./... -v
	cd backend/app-management && go test ./... -v
	cd backend/user-service && go test ./... -v
	cd backend/local-storage && go test ./... -v
	cd backend/message-bus && go test ./... -v

# ─── Utilities ─────────────────────────────────────────────────────────

clean: ## Clean build artifacts
	rm -rf ui/build ui/.svelte-kit ui/node_modules/.vite

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
