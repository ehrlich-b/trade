# TODO

## Core UX - COMPLETE (2025-12-16)

All core issues are fixed. Full test coverage with Go unit tests and Playwright e2e tests.

### Core Issues - ALL FIXED

#### 1. Open Orders Display - DONE
- [x] Show user's pending orders in the book or separate panel
- [x] Add cancel buttons next to each open order
- [x] Added GET /api/orders endpoint to fetch user's open orders
- [x] Created OpenOrders component in center panel

#### 2. Symbol Selector - DONE
- [x] Add dropdown to select trading symbol (even with just "FAKE" for now)
- [x] Header shows selected symbol prominently
- [x] Ready for multi-symbol support

#### 3. Position Display Not Working - FIXED
- [x] Debug why positions show 0 after trades
  - **BUG FOUND**: Only submitter positions updated, not counterparty!
  - **FIXED in server.go**: Now updates BOTH buyer AND seller positions
  - **FIXED in market/maker.go**: MM trades now notify server for counterparty updates
- [x] Verify backend returns correct position data
- [x] Cash/Position/Net Worth should now reflect reality

#### 4. Input Validation - DONE
- [x] Quantity must be > 0, integer - validated with inline error
- [x] Price must be > 0 for limit orders - validated with inline error
- [x] Show error messages inline, not alerts
- [x] Added quick quantity buttons (10, 25, 50, 100)

#### 5. UI Layout Overhaul - DONE
- [x] Header now shows: Symbol Selector | Prices | Account Summary (Cash/Position/P&L/Net Worth) | Status/User
- [x] Account summary always visible in header
- [x] Order form is compact with quick quantity buttons
- [x] Leaderboard/Trades as tabbed view (not both visible)
- [x] Position details in center panel with grid layout
- [x] Cleaner, more professional dark theme

UI enhancements (completed):
- [x] Add "Open Orders" panel showing pending orders with cancel buttons
- [x] Highlight MY orders in the order book with different color (blue background + dot marker)

---

## Recently Fixed (2025-12-16)
- [x] **Persistent sessions**: Sessions now stored in database, survive server restarts
- [x] **401 error handling**: Frontend auto-logs out on expired/invalid tokens
- [x] **Price input validation**: Rounds to 2 decimal places, prevents precision errors
- [x] **Playwright e2e tests**: Comprehensive frontend tests covering OpenOrders, position details, P&L
- [x] **Fixed market maker account errors**: Skip position tracking for market_maker user
- [x] **Database cleanup on test runs**: Fresh DB for each test run
- [x] Test commands: `make test` (Go unit tests), `make test-e2e` (Playwright browser tests)

## Previously Fixed (2025-12-15)
- [x] **Critical position tracking bug**: Both buyer AND seller positions now update on trades
- [x] **Market maker callback**: MM trades now notify server for counterparty position updates
- [x] Database migration system added
- [x] Order cancel authorization (only owner can cancel)
- [x] Rate limiting (100 req/min/IP)
- [x] CORS configurable via -cors flag
- [x] Graceful shutdown with signal handling
- [x] Session expiration enforcement
- [x] Market maker position/P&L tracking
- [x] Self-trade prevention
- [x] WebSocket dead connection pruning
- [x] Store/market package test coverage
- [x] Error boundary for React
- [x] Loading state for order submission

---

## Backlog (After Core UX Fixed)

### Real Data Integration
- [ ] Polygon.io free tier integration
- [ ] Alpaca API integration
- [ ] 15-min delayed feed for market makers
- [ ] Real-time feed tier for players

### Enhanced Features
- [ ] Multiple stocks (after symbol selector works)
- [ ] Stop orders
- [ ] IOC/FOK order types
- [ ] Trade history charts (lightweight-charts)
- [ ] Bot API for player algorithms
- [ ] Keyboard shortcuts (Enter to submit, Esc to cancel)
- [ ] Quick quantity buttons (10, 25, 50, 100, MAX)

### Infrastructure
- [ ] Basic monitoring/logging
- [ ] Performance metrics

---

## Design Principles (Reminder)

1. **Trading Terminal First** - This should feel like a Bloomberg terminal, not a web form
2. **Information Density** - Traders want to see everything at once
3. **Keyboard Friendly** - Power users hate clicking
4. **Real-time Feedback** - Every action should have immediate visual feedback
5. **Position Awareness** - User should ALWAYS know their position and P&L

---

## Open Questions

- Bankruptcy penalty: lockout duration? shame board?
- Market maker personality variations?
- Overnight position rules?
- Position limits needed?
