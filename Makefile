.PHONY: run build test lint migrate-up migrate-down migrate-create docker-up docker-down help

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run the API server
	go run ./cmd/api

build: ## Build the API binary
	go build -o bin/api ./cmd/api

test: ## Run tests with race detector
	go test ./... -v -race -count=1

lint: ## Run linter
	golangci-lint run ./...

migrate-up: ## Run all up migrations
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down: ## Run all down migrations
	migrate -path migrations -database "$$DATABASE_URL" down

migrate-create: ## Create a new migration (usage: make migrate-create name=create_foo)
	migrate create -ext sql -dir migrations -seq $(name)

docker-up: ## Start Docker containers
	docker compose up -d

docker-down: ## Stop Docker containers
	docker compose down
