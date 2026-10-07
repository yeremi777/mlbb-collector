# Load DB_* and APP_* from .env when it exists, and export them to every recipe.
ifneq (,$(wildcard .env))
include .env
export
endif

MIGRATIONS_DIR := internal/database/migrations
# Keyword/value DSN; every value is quoted so an empty or spaced password parses.
DB_DSN = host='$(DB_HOST)' port='$(DB_PORT)' dbname='$(DB_NAME)' user='$(DB_USERNAME)' password='$(DB_PASSWORD)' sslmode='$(DB_SSLMODE)'
GOOSE = goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)"

.DEFAULT_GOAL := help
.PHONY: help seed test test-integration vet fmt tidy migrate-up migrate-down migrate-reset migrate-status migrate-create db-env

help: ## List the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-17s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Dataset

seed: db-env ## Make the public tables match data/
	go run ./cmd/seed

## Code

test: ## Run all tests
	go test ./...

test-integration: db-env ## Run all tests, plus those against the DB_* database in rolled-back transactions
	go test -tags integration -count=1 ./...

vet: ## Report suspicious constructs
	go vet ./...
	go vet -tags integration ./...

fmt: ## Format all Go files
	gofmt -w .

tidy: ## Sync go.mod and go.sum with the imports
	go mod tidy

## Database (goose; recipes are silent so the DSN and its password are never printed)

migrate-up: db-env ## Apply every pending migration
	@$(GOOSE) up

migrate-down: db-env ## Roll back the latest migration
	@$(GOOSE) down

migrate-reset: db-env ## Roll back every migration
	@$(GOOSE) reset

migrate-status: db-env ## Show which migrations are applied
	@$(GOOSE) status

migrate-create: ## Create a timestamped SQL migration: make migrate-create name=<name>
	$(if $(name),,$(error usage: make migrate-create name=<name>))
	goose -dir $(MIGRATIONS_DIR) create $(name) sql

db-env:
	$(foreach v,DB_HOST DB_PORT DB_NAME DB_USERNAME DB_SSLMODE,$(if $($(v)),,$(error $(v) is not set; copy .env.example to .env)))
