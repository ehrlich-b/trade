# CLAUDE.md - Project Instructions

## Project Overview

PvP paper trading game with real order book mechanics. Players trade against each other, not simulated prices. Market manipulation is encouraged.

## Anchor Documents

- `README.md` - Full project vision and architecture
- `TODO.md` - Current tasks and roadmap

## Tech Stack

- **Backend:** Go (single binary, in-memory order book, SQLite persistence)
- **Frontend:** TypeScript/React (embedded in Go binary)
- **Database:** SQLite
- **Real-time:** WebSocket

## Development Commands

**Always use make commands instead of raw go/npm commands.**

```bash
# Check Go code compiles
make check

# Run server
make run

# Run all tests
make test

# Run tests with coverage
make test-cover

# Build frontend only
make frontend

# Full build (frontend + Go binary)
make build

# Run frontend dev server (for hot reload during UI development)
make frontend-dev

# Run e2e tests (starts server automatically)
make test-e2e

# Clean build artifacts
make clean
```

## Code Style

### Go
- Standard Go formatting (gofmt)
- Keep packages small and focused
- Prefer simplicity over abstraction
- Error handling: return errors, don't panic
- Tests in `_test.go` files alongside code

### TypeScript
- Strict mode enabled
- Functional components with hooks
- Keep components small
- Colocate styles with components

## Architecture Principles

1. **In-memory first** - Order book lives in RAM, flush to SQLite periodically
2. **Lost trades are acceptable** - This is a game, not a real exchange
3. **Single process** - Don't over-engineer for scale we don't have
4. **Ship fast** - MVP goal is to test if it's fun, not build perfect infrastructure

## Key Directories

```
cmd/trade/          - Entry point
internal/orderbook/ - Matching engine (start here)
internal/market/    - Synthetic prices, market maker bots
internal/account/   - User accounts, positions, margin
internal/api/       - HTTP + WebSocket handlers
internal/store/     - SQLite persistence
web/                - Frontend
```

## Testing

- Order book matching logic needs thorough unit tests
- Test edge cases: partial fills, price-time priority, empty book
- API tests for order submission/cancellation
- Don't over-test UI initially

## Current Phase

**MVP Complete - Post-MVP Development**

All MVP phases complete (order book, REST API, WebSocket, frontend, synthetic market, auth, daily reset). Frontend is embedded in Go binary for single-binary deployment. Now working through backlog items.
