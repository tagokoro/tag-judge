SHELL := /bin/sh
.DEFAULT_GOAL := help

-include .env

POSTGRES_DB ?= judge
POSTGRES_USER ?= judge_user
POSTGRES_PASSWORD ?= judge_pass
POSTGRES_PORT ?= 5432
PORT ?= 8080

DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
BACKEND_DIR ?= backend
WEB_DIR ?= frontend
MIGRATIONS_DIR ?= $(BACKEND_DIR)/db/migrations
SEED_FILE ?= $(BACKEND_DIR)/db/seeds/development.sql

.PHONY: help
help: ## Show available commands
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: db-up
db-up: ## Start local PostgreSQL
	docker compose up -d postgres

.PHONY: db-down
db-down: ## Stop local dependencies
	docker compose down

.PHONY: db-ps
db-ps: ## Show local dependency status
	docker compose ps

.PHONY: db-setup
db-setup: db-up migrate-up seed ## Start PostgreSQL, run migrations, and load seed data

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back the latest migration
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

.PHONY: migrate-version
migrate-version: ## Show current migration version
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version

.PHONY: migrate-create
migrate-create: ## Create a migration: make migrate-create name=create_table
	@test -n "$(name)" || (echo "usage: make migrate-create name=create_table"; exit 1)
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

.PHONY: seed
seed: ## Load development seed data
	docker compose exec -T postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) < $(SEED_FILE)

.PHONY: sqlc
sqlc: ## Generate sqlc code
	cd $(BACKEND_DIR) && sqlc generate

.PHONY: test
test: ## Run Go tests
	cd $(BACKEND_DIR) && go test ./...

.PHONY: run-api
run-api: ## Run the Go API server (localhost:8080, API only; frontend is a separate process)
	cd $(BACKEND_DIR) && PORT=$(PORT) DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

.PHONY: api-smoke
api-smoke: ## Check API health and public problems
	curl -i http://localhost:$(PORT)/healthz
	curl -s http://localhost:$(PORT)/api/v1/problems

# --- Next.js frontend (separate process on localhost:3000) ---
# Requires Node.js >= 20.9.0 and npm >= 10. See frontend/README.md.
.PHONY: frontend-install
frontend-install: ## Install frontend dependencies from the exact lockfile
	cd $(WEB_DIR) && npm ci

.PHONY: run-frontend
run-frontend: ## Run the Next.js dev server (localhost:3000)
	cd $(WEB_DIR) && npm run dev

.PHONY: frontend-start
frontend-start: ## Serve the production Next.js build (localhost:3000)
	cd $(WEB_DIR) && npm run start

.PHONY: frontend-lint
frontend-lint: ## Run frontend ESLint
	cd $(WEB_DIR) && npm run lint

.PHONY: frontend-typecheck
frontend-typecheck: ## Run frontend TypeScript check
	cd $(WEB_DIR) && npm run typecheck

.PHONY: frontend-build
frontend-build: ## Build the Next.js app
	cd $(WEB_DIR) && npm run build

.PHONY: frontend-check
frontend-check: frontend-lint frontend-typecheck frontend-build ## Run frontend lint, typecheck, and build
