# Load .env so goose (GOOSE_DRIVER/GOOSE_DBSTRING/GOOSE_MIGRATION_DIR) and the
# app see the same config. Missing file is ignored.
-include .env
export

GOOSE_VERSION ?= v3.29.0
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

.PHONY: run build test vet fmt tidy migrate-up migrate-down migrate-status migrate-create

run: ## Run the HTTP server
	go run ./cmd/app

build: ## Build the server binary to bin/app
	go build -o bin/app ./cmd/app

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -w .

tidy: ## Tidy go.mod
	go mod tidy

migrate-up: ## Apply all pending migrations
	$(GOOSE) up

migrate-down: ## Roll back the most recent migration
	$(GOOSE) down

migrate-status: ## Show migration status
	$(GOOSE) status

migrate-create: ## Create a migration: make migrate-create name=create_users
	@test -n "$(name)" || (echo "usage: make migrate-create name=<name>"; exit 1)
	$(GOOSE) create $(name) sql
