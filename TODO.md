# TODO

## Current Focus: MVP

Get to "is this fun?" as fast as possible.

**Goal: Playable state by end of Phase 4** - ACHIEVED

### Phase 5: Synthetic Market
- [x] Random walk price generator
- [x] Market maker bot (quotes around synthetic mid)
- [x] Bot order management (cancel stale quotes, requote)

### Phase 6: Accounts & Auth
- [x] SQLite schema for users
- [x] Simple auth (username/password)
- [x] $1M starting margin per account
- [x] Position tracking per user
- [x] P&L calculation

### Phase 7: Daily Reset
- [x] End-of-day settlement logic
- [x] Bankruptcy detection
- [x] Account reset mechanics
- [x] Persist daily snapshots

---

## Backlog (Post-MVP)

### Real Data Integration
- [ ] Polygon.io free tier integration
- [ ] Alpaca API integration
- [ ] 15-min delayed feed for market makers
- [ ] Real-time feed tier for players

### Enhanced Features
- [ ] Multiple stocks
- [ ] Stop orders
- [ ] IOC/FOK order types
- [x] Leaderboard backend API (`GET /api/leaderboard`)
- [ ] Leaderboard frontend UI
- [ ] Trade history charts
- [ ] Bot API for player algorithms

### Infrastructure
- [x] Embed frontend in Go binary
- [x] Goreleaser for cross-platform builds (linux/darwin/windows, amd64/arm64)
- [x] Pure-Go SQLite (modernc.org/sqlite) for static binaries without CGO
- [ ] Basic monitoring/logging

---

## Known Issues (MVP-acceptable)

- [ ] Position P&L calculation edge cases (closing shorts, position reversal)
- [x] USER_ID regenerates on page refresh (fixed: auth persists in localStorage)
- [x] Trade/Order JSON serialization missing snake_case tags (fixed: added json tags)
- [ ] Trades slice grows unbounded in memory (fine for short sessions)
- [ ] CORS wide open (fine for dev, tighten for prod)

## Open Questions

- Bankruptcy penalty: lockout duration? shame board?
- Market maker personality variations?
- Overnight position rules?
- Position limits needed?
- Initial liquidity seeding strategy?

---

## Completed

### Phase 1: Order Book Engine
- [x] Initialize Go module and project structure
- [x] Implement core order book (bids/asks sorted maps, price-time priority)
- [x] Limit order placement and matching
- [x] Market order execution
- [x] Order cancellation
- [x] Unit tests for matching engine

### Phase 2: REST API
- [x] Basic HTTP server setup (chi router)
- [x] POST /api/orders - submit order
- [x] DELETE /api/orders/:id - cancel order
- [x] GET /api/book - current order book state
- [x] GET /api/trades - recent trades

### Phase 3: WebSocket
- [x] WebSocket endpoint for real-time updates (/ws)
- [x] Broadcast order book changes
- [x] Broadcast trade executions
- [x] Client subscription management

### Phase 4: Minimal Frontend
- [x] Initialize TypeScript/React/Vite project in web/
- [x] Order book depth visualization with depth bars
- [x] Order entry form (limit/market, buy/sell)
- [x] Recent trades list (highlights your trades)
- [x] Current positions display with P&L
- [x] WebSocket connection to backend with auto-reconnect
- [x] Login/Register UI with localStorage persistence
- [x] Server-side position tracking integration

### Phase 8: Testing & Build
- [x] Comprehensive e2e test suite (auth, trading, positions, P&L, WebSocket)
- [x] Static binary with embedded frontend
- [x] Cross-platform builds via goreleaser
- [x] Margin validation (prevents over-leveraging)
- [x] Balance/Net Worth UI display (always visible account summary)
