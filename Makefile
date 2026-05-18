.PHONY: dev build test check lint clean help sync-catalog sync-catalog-dry stage-build test-backend test-ui test-ui-watch

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

# ─── Staging deploy build (Linux/amd64 from any host) ────────────────

stage-build: ## Build hot-swap binaries for Linux/amd64 (CGO svcs from release, #414)
	bash scripts/stage-build.sh

# ─── Catalog sync ─────────────────────────────────────────────────────

sync-catalog: ## Run umbrel-catalog sync locally (writes to community-catalog/)
	cd backend/sync-catalog && go build -o /tmp/sync-catalog .
	/tmp/sync-catalog \
		--source umbrel \
		--output community-catalog \
		--upstream https://github.com/getumbrel/umbrel-apps.git

sync-catalog-dry: ## Dry-run the umbrel-catalog sync (scans + reports, no files written)
	cd backend/sync-catalog && go build -o /tmp/sync-catalog .
	/tmp/sync-catalog \
		--source umbrel \
		--output /tmp/sync-dryrun \
		--upstream https://github.com/getumbrel/umbrel-apps.git \
		--dry-run

# ─── Utilities ─────────────────────────────────────────────────────────

clean: ## Clean build artifacts
	rm -rf ui/build ui/.svelte-kit ui/node_modules/.vite

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
