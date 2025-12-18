# TODO - Arcade Mode Focus

> **See [`mm_design.md`](mm_design.md)** for detailed market making design: redemption mechanism, bot ecosystem, fee structure, failure modes.

## The Game

**Arcade Mode**: Fast-paced trading matches using real historical data, compressed into 10-30 minutes.

### How It Works
1. **Lobby spins** → picks random $SPY day from last 10 years
2. **Price normalized** → scaled to recent closing price (can't tell which day)
3. **Day compressed** → 6.5 market hours → 10/15/30 min match
4. **20+ bots trade** → MMs, momentum, mean reversion, noise traders
5. **Players compete** → real-time leaderboard, best P&L wins
6. **Bell rings** → settlement, rankings, next match

### Why This Works
- **Real dynamics**: Historical data has real volatility, trends, reversals
- **Unpredictable**: Random day + normalization = can't metagame
- **Fuzzy edge**: Even MMs get slightly fuzzed reference (no perfect arb)
- **Fast iteration**: Play 4-6 matches per hour, learn quickly
- **Always available**: 24/7, no market hours needed

---

## Match Structure

```
LOBBY      - Spinning to select historical day
           - Players join, see countdown to start

MATCH START - Bell rings
            - "Trading Day: ???" (day hidden)
            - Everyone at $1M, clock starts

TRADING    - 10/15/30 min depending on match type
            - 1 real second ≈ 13-39 market seconds
            - Bots active, prices moving
            - Real-time leaderboard updating

WARNING    - "90 SECONDS" / "FINAL 30" alerts
            - Urgency intensifies

SETTLEMENT - Bell rings, trading halts
            - Positions marked to final price
            - P&L calculated, rankings shown

NEXT MATCH - Brief intermission, then new day spins
```

---

## Phase 1: Core Systems (BUILD THIS)

### 1.1 Historical Data Pipeline
- [ ] Polygon.io integration (one call per match)
- [ ] Fetch random $SPY day (last 10 years)
- [ ] Normalize prices to recent close (hide which day)
- [ ] Store/cache fetched days locally
- [ ] Time scaling: map real seconds → market time

### 1.2 Match Engine
- [ ] Match state machine (LOBBY → TRADING → SETTLEMENT)
- [ ] Configurable duration (10/15/30 min)
- [ ] Clock + time acceleration
- [ ] Trading halt at match end
- [ ] Settlement calculation
- [ ] WebSocket broadcast of match state

### 1.3 Price Feed System
- [ ] "True price" from historical data (normalized)
- [ ] MM reference price (slightly fuzzed from true)
- [ ] Player-visible mid price (from order book)
- [ ] Price ticks at accelerated rate

### 1.4 Bot Ecosystem (20+ bots)

**Market Makers (4 bots)** - liquidity providers, exploitable:
- [ ] Tight MM: 5¢ spread, size 20, fast quotes
- [ ] Wide MM: 30¢ spread, size 200, slow quotes
- [ ] Adaptive MM: spread widens with volatility/inventory
- [ ] Nervous MM: pulls quotes on big moves, slow to return

MM Behavior:
- [ ] Quote around fuzzed reference price
- [ ] Inventory skew (long → lower quotes, short → higher)
- [ ] Adverse selection tracking (widen after being picked off)
- [ ] Position limits (stop quoting one side at max)

**Directional Traders (8 bots)** - create price movement:
- [ ] Momentum Fast (×2): chases 10-second trends
- [ ] Momentum Slow (×2): chases 1-minute trends
- [ ] Mean Reversion (×2): fades 20¢+ moves
- [ ] Breakout (×2): jumps on range breaks

**Noise Traders (8 bots)** - chaos and volume:
- [ ] Random (×4): random market orders, small size
- [ ] Panic (×2): overreacts to price moves
- [ ] Slow (×2): random but infrequent, larger size

### 1.5 Match UI
- [ ] **BIG TIMER** - center screen, countdown
- [ ] **Match type badge** - "10 MIN MATCH"
- [ ] **Live leaderboard** - P&L rankings updating
- [ ] **Trade tape** - all trades, all participants
- [ ] **Price display** - current mid, high/low of session
- [ ] **"FINAL 30"** - dramatic countdown mode
- [ ] **Settlement screen** - your rank, P&L, stats
- [ ] **Bell sounds** - start/end audio

---

## Phase 2: Competition

### 2.1 Leaderboards
- [ ] In-match live rankings
- [ ] Match history (your results)
- [ ] All-time stats (matches played, win rate, avg P&L)
- [ ] Streak tracking

### 2.2 Match Lobby
- [ ] See upcoming match countdown
- [ ] Join queue for next match
- [ ] Match type selection (10/15/30 min)

---

## Phase 3: Polish

- [ ] Keyboard shortcuts
- [ ] Sound effects (trades, alerts, bells)
- [ ] Mini price chart
- [ ] Mobile responsive
- [ ] Multiple symbols (beyond $SPY)

---

## Technical Notes

### Polygon API Usage
```
GET /v2/aggs/ticker/SPY/range/1/minute/{date}/{date}
- Returns ~390 minute bars per day
- One API call per match
- Free tier: 5 calls/min (enough for lobby spinning)
- Cache fetched days locally
```

### Price Normalization
```go
// Example: Historical day had SPY at $380, current close is $480
scaleFactor := currentClose / historicalOpen
normalizedPrice := historicalPrice * scaleFactor
// Now all prices are in current-ish range
```

### Time Acceleration
```
10 min match: 1 real sec = 39 market sec (390 min / 10 min)
15 min match: 1 real sec = 26 market sec
30 min match: 1 real sec = 13 market sec
```

### MM Fuzzing
```go
// MMs don't see exact true price
mmReference := truePrice + randomNoise(-0.10, +0.10)
// Prevents perfect arbitrage, creates edge for observant players
```

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    MATCH ENGINE                      │
│  ┌─────────┐  ┌──────────┐  ┌─────────────────────┐ │
│  │ Polygon │→ │Historical│→ │   Time Scaler       │ │
│  │   API   │  │  Cache   │  │ (real → market time)│ │
│  └─────────┘  └──────────┘  └──────────┬──────────┘ │
│                                        │            │
│                              ┌─────────▼─────────┐  │
│                              │   True Price      │  │
│                              │   (normalized)    │  │
│                              └─────────┬─────────┘  │
│                    ┌───────────────────┼───────────┐│
│              ┌─────▼─────┐       ┌─────▼─────┐     ││
│              │ MM Fuzzed │       │ Bot Feed  │     ││
│              │ Reference │       │ (exact)   │     ││
│              └─────┬─────┘       └─────┬─────┘     ││
│                    │                   │           ││
│              ┌─────▼─────┐       ┌─────▼─────┐     ││
│              │    MMs    │       │   Bots    │     ││
│              │  (4 bots) │       │ (16 bots) │     ││
│              └─────┬─────┘       └─────┬─────┘     ││
│                    │                   │           ││
│                    └─────────┬─────────┘           ││
│                              │                     ││
│                    ┌─────────▼─────────┐           ││
│                    │    ORDER BOOK     │←── Players││
│                    │    (existing)     │           ││
│                    └─────────┬─────────┘           ││
│                              │                     ││
│                    ┌─────────▼─────────┐           ││
│                    │    WebSocket      │           ││
│                    │    Broadcast      │           ││
│                    └───────────────────┘           ││
└─────────────────────────────────────────────────────┘
```

---

## What We're NOT Building (Yet)

- Live mode (real-time market data)
- Multiple stocks
- Options
- Fancy charts
- Mobile app
- Player bot API

---

## Success Criteria

**Is it fun to play a 10-minute match against 20 bots?**

- Does the market feel alive?
- Can skilled players consistently beat bots?
- Is there tension as the clock runs down?
- Do you want to play "just one more match"?
