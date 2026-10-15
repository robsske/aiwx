.PHONY: build run test lint clean migrate-up migrate-down dev up down smoke-test

# ─── Variables ─────────────────────────────────────────────────────────────────
BINARY=bin/aiwx
GO=go
GOLANGCI=golangci-lint

# ─── Build ────────────────────────────────────────────────────────────────────
build:
	$(GO) build -o $(BINARY) ./cmd/aiwx

run: build
	DATABASE_URL=$(DATABASE_URL) ./$(BINARY)

dev:
	./scripts/dev.sh

up:
	docker-compose up -d
	@echo "Waiting for PostgreSQL..."
	@until docker exec aiwx-postgres pg_isready -U aiwx 2>/dev/null; do sleep 1; done
	@echo "✅ Services are ready"

down:
	docker-compose down

smoke-test:
	./scripts/smoke-test.sh

# ─── Test ─────────────────────────────────────────────────────────────────────
test:
	$(GO) test ./... -v -race -count=1

test-short:
	$(GO) test ./... -short -count=1

test-coverage:
	$(GO) test ./... -race -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html

# ─── Lint ─────────────────────────────────────────────────────────────────────
lint:
	$(GOLANGCI) run ./...

lint-fix:
	$(GOLANGCI) run --fix ./...

fmt:
	$(GO) fmt ./...
	$(GOLANGCI) run --fix ./...

# ─── Migrations ───────────────────────────────────────────────────────────────
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-version:
	migrate -path migrations -database "$(DATABASE_URL)" version

migrate-force:
	migrate -path migrations -database "$(DATABASE_URL)" force $(VERSION)

# ─── Generate ─────────────────────────────────────────────────────────────────
generate:
	$(GO) generate ./...

# ─── Clean ────────────────────────────────────────────────────────────────────
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# ─── Docker ───────────────────────────────────────────────────────────────────
docker-build:
	docker build -t aiwx:latest .

# ─── Help ─────────────────────────────────────────────────────────────────────
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  run            - Build and run the server"
	@echo "  dev            - Run in development mode"
	@echo "  test           - Run all tests with race detection"
	@echo "  test-short     - Run tests in short mode"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run linter"
	@echo "  lint-fix       - Run linter with auto-fix"
	@echo "  fmt            - Format code"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down   - Rollback database migrations"
	@echo "  clean          - Remove build artifacts"
	@echo "  help           - Show this help"
