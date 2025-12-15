# trade

A PvP paper trading game where players actually trade against each other through a real order book — not the usual "everyone simulates their own spreadsheet" approach.

## The Vision

Most paper trading games are boring because there's no market. You're not trading against anyone, you're just simulating price movements in isolation. This is different: **"wrestling in the mud and one of you has a box cutter"** — a genuinely adversarial trading environment where pump & dumps, painting the tape, front-running, and every other normally-forbidden technique is fair game.

The core insight: market makers see 15-minute delayed data. Smart players with live data (eventually from real APIs) can front-run them. This asymmetry creates actual alpha for skilled players.

## Core Game Mechanics

### Account Model
- Players start each day with **$1M margin**
- End-of-day settlement: you must be able to cover your $1M obligation
- Bankruptcy = account wipes, you're done for the day (or some penalty system)
- Can redeem fake USD or hold stock positions overnight (TBD on overnight mechanics)

### The Order Book
- Real limit order book with price-time priority matching
- Players trade against each other, not against simulated prices
- All standard order types: limit, market, (eventually) stop, IOC, FOK, etc.
- The "price" of a stock IS the order book — no external oracle except for seeding/market makers

### Market Makers
- Bot accounts that provide liquidity based on **15-minute delayed data**
- Intentionally exploitable: players with faster data can pick them off
- Keeps the market liquid when player activity is low
- Could have different "personalities" (tight spreads, wide spreads, different sizes)

### Data Sources (Phase Evolution)

**Phase 1 (MVP):** Completely fake data
- Generate synthetic price movements (random walk, mean reversion, etc.)
- Single stock: "$FAKE" or similar
- Focus on proving the order book mechanics are fun

**Phase 2:** Delayed real data
- Free APIs: Polygon.io (5 calls/min), Alpaca (200 req/min), Finnhub
- Market makers trade on this 15-min delayed feed
- Players can still only see the same delayed data (no real-time yet)

**Phase 3:** Information asymmetry
- Some players get real-time data (paid feature? earned? achievement?)
- Market makers stay on delayed data — now there's actual edge

**Phase 4:** Multi-stock, options, etc.

## Technical Architecture

### Stack
- **Backend:** Go
  - Single binary deployment
  - In-memory order book (flush to SQLite ~1/sec)
  - Trades can be lost between flushes — this is acceptable for a game
  - WebSocket for real-time order book updates
  - REST for account management, order submission

- **Frontend:** TypeScript (embedded in Go binary)
  - React or Svelte (your choice)
  - Real-time order book visualization
  - Charts (TradingView lightweight-charts or similar)
  - Order entry panel
  - Position/P&L tracking

- **Database:** SQLite
  - User accounts, authentication
  - End-of-day position snapshots
  - Trade history (for leaderboards, auditing)
  - Periodic order book snapshots

### Order Book Implementation
Consider using or adapting `github.com/i25959341/orderbook`:
- 300k+ matches/sec
- Price-time priority
- Limit + market orders
- Order cancellation
- JSON marshalling built-in

Or build from scratch — it's not that complex:
- Two sorted maps (bids descending, asks ascending)
- Each price level is a FIFO queue
- Match incoming orders against opposite side

### Key Technical Decisions

**In-Memory First:**
```
Order arrives → Match in memory → Broadcast fills via WebSocket → Eventually flush to SQLite
```
Lost trades between flushes are acceptable. This is a game, not a real exchange.

**Single Process (Initially):**
- Don't over-engineer for scale you don't have
- One server can handle thousands of concurrent traders
- Horizontal scaling can come later if needed

**No Real Money:**
- No cash out mechanism
- Leaderboards and bragging rights only (for now)
- This keeps regulatory complexity at zero

## MVP Definition

The absolute minimum to test if this is fun:

1. **One fake stock** with synthetic price feed driving market maker bots
2. **Working order book** — limit orders, market orders, cancels
3. **Basic web UI** — order entry, order book depth, recent trades, your positions
4. **User accounts** — simple auth (magic link? OAuth? username/password?)
5. **$1M daily margin** — account resets each day
6. **Real-time updates** — WebSocket for order book changes, fills

NOT in MVP:
- Real market data integration
- Options
- Multiple stocks
- Bot API
- Mobile
- Leaderboards (nice to have but not essential for fun test)

## Project Structure

```
trade/
├── cmd/
│   └── trade/
│       └── main.go           # Entry point
├── internal/
│   ├── orderbook/            # Matching engine
│   ├── market/               # Market data, market makers
│   ├── account/              # User accounts, positions
│   ├── api/                  # HTTP/WebSocket handlers
│   └── store/                # SQLite persistence
├── web/                      # TypeScript frontend
│   ├── src/
│   ├── package.json
│   └── ...
├── go.mod
├── go.sum
└── CLAUDE.md
```

## Commands Reference

```bash
# Run the server
make run

# Run tests
make test

# Build frontend only
make frontend

# Full build (frontend + Go binary)
make build

# Check Go code compiles
make check
```

## Questions to Resolve

- What happens if you go bankrupt? Locked out for the day? Penalty? Immediate reset with shame?
- How do market maker bots behave? Simple bid/ask around mid? React to order flow?
- Overnight positions: allowed? Interest charges? Forced liquidation?
- Should there be position limits?
- How do we seed initial liquidity on day 1?

---

**Remember:** The goal is to find out if this is FUN. Ship the MVP fast, play it, iterate. Don't build the perfect exchange — build the minimum thing that lets you pump and dump a fake stock against your friends.
