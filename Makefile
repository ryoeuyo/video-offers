GOOSE_DRIVER  ?= postgres
GOOSE_DBSTRING ?= $(shell grep -E '^DATABASE_URL=' .env 2>/dev/null | cut -d= -f2-)
GOOSE_MIGRATION_DIR ?= ./migrations
export GOOSE_DRIVER GOOSE_DBSTRING GOOSE_MIGRATION_DIR

.PHONY: run build test lint db-up db-down migrate-up migrate-down migrate-status migrate-new tidy

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./... -race

lint:
	golangci-lint run

tidy:
	go mod tidy

db-up:
	docker compose up -d db

db-down:
	docker compose down

migrate-up:
	go tool goose up

migrate-down:
	go tool goose down

migrate-status:
	go tool goose status

# make migrate-new n=add_offer_reactions
migrate-new:
	go tool goose -s create $(n) sql
