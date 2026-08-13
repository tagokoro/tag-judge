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
CLOUDFLARE_DIR ?= cloudflare
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
run-api: ## Run the API server
	cd $(BACKEND_DIR) && PORT=$(PORT) DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

.PHONY: api-smoke
api-smoke: ## Check API health and public problems
	curl -i http://localhost:$(PORT)/healthz
	curl -s http://localhost:$(PORT)/api/v1/problems

.PHONY: cf-install
cf-install: ## Install Cloudflare Worker dependencies
	cd $(CLOUDFLARE_DIR) && npm install

.PHONY: cf-typecheck
cf-typecheck: ## Type-check Cloudflare Workers
	cd $(CLOUDFLARE_DIR) && npm run typecheck

.PHONY: cf-d1-migrate-local
cf-d1-migrate-local: ## Apply Cloudflare D1 migrations locally
	cd $(CLOUDFLARE_DIR) && npm run d1:migrate:local

.PHONY: cf-dev-api
cf-dev-api: ## Run Cloudflare API Worker locally
	cd $(CLOUDFLARE_DIR) && npm run dev:api

.PHONY: cf-dev-consumer
cf-dev-consumer: ## Run Cloudflare Consumer Worker locally
	cd $(CLOUDFLARE_DIR) && npm run dev:consumer
