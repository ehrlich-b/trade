.PHONY: all build build-go run test test-e2e clean dev frontend check release snapshot

# Default target
all: build

# Build everything (Go + frontend)
build: frontend build-go

# Build Go binary only
build-go:
	go build -o bin/trade ./cmd/trade

# Check that Go code compiles
check:
	go build ./...

# Run the server
run:
	go run ./cmd/trade

# Run with hot reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air

# Run all tests
test:
	go test ./...

# Run tests with coverage
test-cover:
	go test -cover ./...

# Run e2e tests (builds and starts server automatically)
test-e2e:
	cd web && npm run test

# Build frontend
frontend:
	cd web && npm install && npm run build

# Install frontend dependencies only
frontend-deps:
	cd web && npm install

# Run frontend dev server
frontend-dev:
	cd web && npm run dev

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist/

# Format code
fmt:
	go fmt ./...

# Lint
lint:
	golangci-lint run

# Build snapshot release (for testing)
snapshot:
	goreleaser release --snapshot --clean

# Build release (requires git tag)
release:
	goreleaser release --clean
