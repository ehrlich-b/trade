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

## Phase 1: Core Systems ✅ COMPLETE

### 1.1 Historical Data Pipeline ✅
- [x] Polygon.io integration (one call per match)
- [x] Fetch random $SPY day (last 10 years)
- [x] Normalize prices to recent close (hide which day)
- [x] Store/cache fetched days locally
- [x] Time scaling: map real seconds → market time

### 1.2 Match Engine ✅
- [x] Match state machine (LOBBY → TRADING → SETTLEMENT)
- [x] Configurable duration (10/15/30 min)
- [x] Clock + time acceleration
- [x] Trading halt at match end
- [x] Settlement calculation
- [x] WebSocket broadcast of match state

### 1.3 Price Feed System ✅
- [x] "True price" from historical data (normalized)
- [x] MM reference price (slightly fuzzed from true)
- [x] Player-visible mid price (from order book)
- [x] Price ticks at accelerated rate

### 1.4 Bot Ecosystem (24 bots) ✅

**Market Makers (4 bots)** - liquidity providers, exploitable:
- [x] Tight MM: 5¢ spread, size 20, fast quotes
- [x] Wide MM: 25¢ spread, size 200, slow quotes
- [x] Adaptive MM: spread widens with volatility/inventory
- [x] Nervous MM: pulls quotes on big moves, slow to return

MM Behavior:
- [x] Quote around fuzzed reference price
- [x] Inventory skew (long → lower quotes, short → higher)
- [x] Position limits (stop quoting one side at max)

**Directional Traders (8 bots)** - create price movement:
- [x] Momentum Fast (×2): chases 10-second trends
- [x] Momentum Slow (×2): chases 1-minute trends
- [x] Mean Reversion (×2): fades 20¢+ moves
- [x] Breakout (×2): jumps on range breaks

**Noise Traders (8 bots)** - chaos and volume:
- [x] Random Small (×4): random market orders, small size
- [x] Random Large (×2): infrequent, larger size
- [x] Panic (×2): overreacts to price moves

**Mandated Agents (4 bots)** - create natural flow:
- [x] TWAP Buyer/Seller: steady execution over time
- [x] Opportunistic Buyer: waits for good prices
- [x] Desperate Seller: urgent execution

### 1.5 Match UI ✅
- [x] **BIG TIMER** - center screen, countdown
- [x] **Match type badge** - "10 MIN MATCH"
- [x] **Live leaderboard** - P&L rankings updating
- [x] **"FINAL 30"** - dramatic countdown mode
- [x] **Settlement screen** - your rank, P&L, stats
- [x] **Lobby screen** - join matches, see participants
- [ ] **Bell sounds** - start/end audio (future)

---

## Phase 2: Competition (IN PROGRESS)

### 2.1 Leaderboards
- [x] In-match live rankings (MatchLeaderboard component)
- [x] Match history (store.GetUserMatchHistory)
- [x] All-time stats (matches played, win rate, avg P&L) - user_stats table
- [x] Streak tracking (current_streak, best_streak in user_stats)

### 2.2 Match Scheduler ✅
- [x] Automatic match rotation (game.Scheduler)
- [x] Match persistence (store.SaveMatch)
- [x] User stats updates on match end

### 2.3 Match Lobby
- [x] See upcoming match countdown (MatchLobby component)
- [x] Join matches (scheduler.JoinMatch)
- [ ] Match type selection (10/15/30 min) - UI pending

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
