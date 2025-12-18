# Market Design: The Redemption Anchor Model

## Core Insight: The ETF Creation/Redemption Mechanism

The breakthrough idea: **Everyone can see the redemption price, and anyone can redeem/create shares at that price minus a small fee (0.5%).**

This is exactly how real ETFs work. SPY trades at ~$480, but its Net Asset Value (NAV) is calculated from the 500 underlying stocks. Authorized Participants can:
- **Create shares**: Deliver underlying basket → receive ETF shares
- **Redeem shares**: Deliver ETF shares → receive underlying basket

This keeps SPY within a tight band of NAV. If SPY trades at $481 (1% premium), APs create shares and sell them until the premium closes.

### Why This Solves Our Problem

| Problem | Solution |
|---------|----------|
| "If everyone knows the reference price, why trade elsewhere?" | The 0.5% fee creates a **band of uncertainty** where the market price can float |
| "MMs are infinitely exploitable" | MMs only need to stay inside the redemption band to be safe |
| "Price has no anchor" | Redemption price IS the anchor—but it's expensive to use |
| "No arbitrage opportunities" | Price can deviate up to 0.5% before arb kicks in |

---

## The Three-Price Model

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│    REDEMPTION CEILING (+0.5%)  ────────────────────────     │
│         ↑                                                   │
│         │  Players can SELL shares back at NAV - 0.5%       │
│         │  (effectively caps how high price can go)         │
│         │                                                   │
│    ════ NAV (Historical/Reference Price) ════════════════   │
│         │                                                   │
│         │  The "gravitational center"                       │
│         │  Everyone can see this                            │
│         │                                                   │
│         ↓                                                   │
│    CREATION FLOOR (-0.5%)  ─────────────────────────────    │
│         Players can BUY new shares at NAV + 0.5%            │
│         (effectively caps how low price can go)             │
│                                                             │
│    ═══════════════════════════════════════════════════════  │
│                                                             │
│    MARKET PRICE (Order Book Mid)                            │
│         Floats freely within the band                       │
│         Can temporarily breach band during volatility       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### The Math: Dynamic Fee

The redemption fee starts low but increases asymptotically as cumulative redemption volume grows over the match. This prevents infinite arbitrage abuse while keeping early redemptions cheap.

```go
type RedemptionEngine struct {
    nav              int64   // Current NAV in cents
    cumulativeVolume int64   // Total shares redeemed/created this match
    matchDuration    float64 // Match length in minutes
    elapsedTime      float64 // Time elapsed in minutes
}

const (
    BaseFee    = 0.005  // 0.5% starting fee
    MaxFee     = 0.03   // 3% asymptotic cap
    VolumeHalf = 50000  // Volume at which fee is halfway to max
)

// CurrentFee: increases with cumulative volume * time pressure
func (r *RedemptionEngine) CurrentFee() float64 {
    // Time factor: fee pressure increases as match progresses
    timeFactor := 1.0 + (r.elapsedTime / r.matchDuration)

    // Volume factor: asymptotic rise toward MaxFee
    // fee = BaseFee + (MaxFee - BaseFee) * (vol / (vol + halfVol))
    effectiveVol := float64(r.cumulativeVolume) * timeFactor
    volRatio := effectiveVol / (effectiveVol + float64(VolumeHalf))

    return BaseFee + (MaxFee - BaseFee) * volRatio
}

// CreationPrice: buy new shares at NAV + fee
func (r *RedemptionEngine) CreationPrice() int64 {
    return int64(float64(r.nav) * (1 + r.CurrentFee()))
}

// RedemptionPrice: sell shares at NAV - fee
func (r *RedemptionEngine) RedemptionPrice() int64 {
    return int64(float64(r.nav) * (1 - r.CurrentFee()))
}
```

**Fee Curve Example (15-min match):**

| Cumulative Volume | Time | Fee | Creation @ $100 NAV | Redemption @ $100 NAV |
|-------------------|------|-----|---------------------|----------------------|
| 0 | 0:00 | 0.50% | $100.50 | $99.50 |
| 10,000 | 5:00 | 0.85% | $100.85 | $99.15 |
| 30,000 | 10:00 | 1.60% | $101.60 | $98.40 |
| 100,000 | 14:00 | 2.50% | $102.50 | $97.50 |

**The message:** First to arb gets cheap redemption. Heavy/late arb pays the tax.

**UI Display:**
```
REDEMPTION MECHANISM
  NAV: $100.00
  Current Fee: 1.2%
  Create at: $101.20  |  Redeem at: $98.80

  [CREATE SHARES]  [REDEEM SHARES]
```

---

## Buy-Side and Sell-Side Agents

### The Mandate Model

Instead of reactive bots that just quote around a price, we introduce **agents with mandates**. They have a job to do: accumulate or liquidate a specific quantity over the match.

```go
type ExecutionAgent struct {
    mandate     int64         // +5000 = must BUY 5000 shares, -3000 = must SELL 3000
    filled      int64         // how much we've done
    deadline    time.Duration // must complete by this time
    urgency     float64       // 0.0 = patient, 1.0 = desperate
    strategy    Strategy      // TWAP, VWAP, Opportunistic, etc.
}
```

### Why Mandates Create Real Markets

Real institutional traders don't trade to "make money on the spread." They trade because:
- A pension fund needs to rebalance
- An index is reconstituting
- A fund had redemptions and must liquidate
- A fund had inflows and must deploy capital

**Mandated agents create natural supply/demand that isn't arbitrage.**

### Volume Distribution

At match start, we calculate expected volume from historical data. The total volume target is divided evenly among mandated agents, with random buy/sell assignment.

```go
func SpawnAgentsForMatch(totalVolumeTarget int64, agentCount int) []*ExecutionAgent {
    agents := []*ExecutionAgent{}

    // Each agent gets equal share of volume
    volumePerAgent := totalVolumeTarget / int64(agentCount)

    // Track net to keep roughly balanced
    var netDirection int64

    for i := 0; i < agentCount; i++ {
        // Bias toward balance, but allow some randomness
        var side int64 = 1 // buy
        if netDirection > 0 || (netDirection == 0 && rand.Float64() < 0.5) {
            side = -1 // sell
        }
        netDirection += side

        // Randomize strategy and urgency
        strategy := randomStrategy() // TWAP, VWAP, Opportunistic
        urgency := 0.2 + rand.Float64()*0.6 // 0.2 to 0.8

        agents = append(agents, &ExecutionAgent{
            mandate:  side * volumePerAgent,
            strategy: strategy,
            urgency:  urgency,
        })
    }

    return agents
}
```

**Example: 15-min match, 200k expected volume, 8 agents:**

| Agent | Mandate | Strategy | Urgency |
|-------|---------|----------|---------|
| 1 | +25,000 (buy) | TWAP | 0.3 |
| 2 | -25,000 (sell) | VWAP | 0.7 |
| 3 | +25,000 (buy) | Opportunistic | 0.4 |
| 4 | -25,000 (sell) | TWAP | 0.5 |
| 5 | -25,000 (sell) | VWAP | 0.2 |
| 6 | +25,000 (buy) | Opportunistic | 0.6 |
| 7 | +25,000 (buy) | TWAP | 0.8 |
| 8 | -25,000 (sell) | VWAP | 0.4 |

Net: 0 (balanced). Each agent has same size, different behavior.

### Execution Strategies

**TWAP (Time-Weighted Average Price):**
```
Slice mandate into equal chunks over remaining time
If mandate = 5000 and 10 minutes remain: trade 500/minute
Patient, predictable, exploitable by players who spot the pattern
```

**VWAP (Volume-Weighted Average Price):**
```
Trade proportionally to expected volume curve
More aggressive during high-volume periods
Tries to minimize market impact
```

**Opportunistic:**
```
Wait for favorable prices (inside the band)
Pounce when market moves in favorable direction
More unpredictable, harder to front-run
```

**Desperate (high urgency):**
```
"Just get it done"
Market orders, crosses the spread
Creates volatility spikes at mandate deadlines
```

---

## The Reflexive Price Model

From Gemini research: bots should calculate an Internal Fair Value (IFV) that blends multiple signals.

```go
type InternalFairValue struct {
    nav         int64   // Historical/redemption reference
    bookMid     int64   // Order book midpoint
    lastTrade   int64   // Most recent trade price
    inventory   int64   // Bot's current position
}

func (ifv *InternalFairValue) Calculate(weights Weights) int64 {
    // Base: weighted average of signals
    base := weights.Nav * float64(ifv.nav) +
            weights.Book * float64(ifv.bookMid) +
            weights.Trade * float64(ifv.lastTrade)

    // Inventory skew (Avellaneda-Stoikov)
    // If long, lower our fair value to encourage selling
    skew := float64(ifv.inventory) * weights.InventoryPenalty

    return int64(base - skew)
}
```

### Bot Personalities via Weights

| Bot Type | Nav Weight | Book Weight | Trade Weight | Inventory Penalty |
|----------|------------|-------------|--------------|-------------------|
| Tight MM | 0.2 | 0.5 | 0.3 | High (aggressive rebalancing) |
| Wide MM | 0.6 | 0.2 | 0.2 | Low (tolerates inventory) |
| Adaptive MM | 0.3 | 0.3 | 0.2 | Dynamic (increases with position) |
| Nervous MM | 0.1 | 0.6 | 0.3 | Very High (panics easily) |

---

## Bot Ecosystem Redesign

### Tier 1: Liquidity Providers (Market Makers)

These quote continuously around their IFV. They're the "furniture" of the market.

| Bot | Spread | Size | Latency | Personality |
|-----|--------|------|---------|-------------|
| **Tight MM** | 5¢ | 20 | 50ms | Fast, paranoid, widens on toxicity |
| **Wide MM** | 25¢ | 200 | 2000ms | Slow, dumb, "stale quote" target |
| **Adaptive MM** | 5-30¢ | 50-150 | 200ms | Inventory-aware, visible "lean" |
| **Nervous MM** | 10-50¢ | 30 | 100ms | VPIN-sensitive, disappears in volatility |

### Tier 2: Mandated Executors (Buy/Sell Side)

These have jobs to do. They create natural order flow.

| Bot | Role | Strategy | Behavior |
|-----|------|----------|----------|
| **Patient Buyer** | Pension rebalancing | TWAP | Steady bid pressure, predictable |
| **Patient Seller** | Fund redemption | TWAP | Steady offer pressure, predictable |
| **Opportunistic Buyer** | Value fund | Limit orders | Bids aggressively on dips |
| **Opportunistic Seller** | Profit taker | Limit orders | Offers on rallies |
| **Desperate Buyer** | Mandate deadline | Market orders | Spikes price near end of match |
| **Desperate Seller** | Margin call | Market orders | Crashes price near end of match |

### Tier 3: Reactive Traders (Momentum/Mean Reversion)

These react to price movements created by Tier 1 and 2.

| Bot | Trigger | Behavior |
|-----|---------|----------|
| **Momentum Fast** | 10-second trend | Chases, amplifies moves |
| **Momentum Slow** | 1-minute trend | Lags behind, provides exit liquidity |
| **Mean Reversion** | 20¢+ deviation from NAV | Fades moves, pulls toward NAV |
| **Breakout** | Range expansion | Jumps on volatility spikes |

### Tier 4: Noise (Market Texture)

Random activity that provides cover for informed trading.

| Bot | Behavior |
|-----|----------|
| **Random Small** | Random market orders, 5-20 shares |
| **Random Large** | Infrequent, 100-500 share chunks |
| **Panic** | Overreacts to price moves, creates cascades |

---

## The Redemption Mechanism in Practice

### Player Actions

```
BUY FROM MARKET:  Lift the ask, pay market price
SELL TO MARKET:   Hit the bid, receive market price

CREATE SHARES:    Pay NAV + 0.5%, receive new shares (from nothing)
REDEEM SHARES:    Surrender shares, receive NAV - 0.5% (shares destroyed)
```

### When Redemption Makes Sense

**Scenario: Market price is $99.20, NAV is $100.00**

- Redemption price: $99.50
- Market bid: $99.20
- **Action**: Sell to redemption mechanism (+$0.30/share vs market)

**Scenario: Market price is $100.80, NAV is $100.00**

- Creation price: $100.50
- Market ask: $100.80
- **Action**: Create shares, sell to market (+$0.30/share)

### Arbitrage Bounds

The redemption mechanism creates hard bounds:
- Price can't stay above NAV + 0.5% (arb: create and sell)
- Price can't stay below NAV - 0.5% (arb: buy and redeem)

But within that 1% band, the market is "real"—driven by order flow, mandates, and player skill.

---

## Alpha Opportunities for Players

### 1. Front-Running Mandated Agents

If you spot a TWAP buyer (steady bid every 30 seconds), you can:
- Buy ahead of them
- Let their buying push price up
- Sell into the rally

### 2. Fade the Extremes

When market approaches redemption bounds:
- At +0.4% premium: Sell (mean reversion likely)
- At -0.4% discount: Buy (mean reversion likely)

### 3. Momentum Ignition

Place aggressive orders to trigger Momentum bots:
- Buy aggressively → Momentum bots pile in → Sell to them
- Requires reading when momentum bots are "primed"

### 4. Inventory Reading

Watch MM quote asymmetry:
- If Adaptive MM bid is 2¢ higher than expected: They're short, want to buy
- Trade with their "need"

### 5. Toxicity Games

Recognize when Nervous MM is about to pull quotes:
- High VPIN environment
- Big one-sided flow
- When they disappear, liquidity craters → opportunity for the bold

### 6. End-of-Match Desperation

Mandated agents with unfinished business get desperate:
- If Patient Buyer still has 2000 shares to buy with 60 seconds left
- They'll become Desperate Buyer → price spike incoming

---

## Match Flow

```
T-60s   LOBBY
        - Historical day selected, NAV established
        - Mandated agents spawned with volume targets
        - Players see: "SPY Arcade - 15 MIN MATCH - NAV: $482.35"

T-30s   PRE-MARKET
        - MMs place initial quotes around NAV
        - Mandated agents calculate execution schedules
        - Players can place limit orders

T=0     BELL RINGS
        - Continuous trading begins
        - NAV ticks according to historical data
        - Agents execute their mandates

T+7m    HALFTIME
        - Mandated agents check progress
        - Urgency increases for those behind schedule

T+12m   WARNING
        - "3 MINUTES REMAINING"
        - Desperate agents emerge
        - Volatility typically spikes

T+14m   FINAL MINUTE
        - Mandated agents in panic mode
        - Players scrambling to close positions

T+15m   BELL RINGS
        - Trading halts
        - Positions marked to final NAV
        - P&L calculated
```

---

## Starting Positions: The Inherited Portfolio

Players don't start with $1M cash. They start with $1M in **mixed assets**—a random split between cash and shares valued at the opening NAV.

### The Mechanic

```go
type StartingPosition struct {
    cash       int64   // In cents
    shares     int64   // Number of shares
    shareValue int64   // NAV at match start (cents per share)
}

func GenerateStartingPosition(totalValue int64, nav int64) StartingPosition {
    // Random allocation between 20% and 80% in shares
    sharePct := 0.20 + rand.Float64()*0.60 // 0.20 to 0.80

    shareValue := int64(float64(totalValue) * sharePct)
    shares := shareValue / nav
    cash := totalValue - (shares * nav)

    return StartingPosition{
        cash:       cash,
        shares:     shares,
        shareValue: nav,
    }
}
```

**Example at NAV = $100.00, Total = $1,000,000:**

| Player | Share % | Shares | Cash |
|--------|---------|--------|------|
| Alice | 73% | 7,300 | $270,000 |
| Bob | 28% | 2,800 | $720,000 |
| Carol | 55% | 5,500 | $450,000 |

### Why This Works

1. **Kills the cold start** - No "everyone sits in cash." You have exposure from the bell.

2. **Forces adaptation** - Real traders inherit positions. Skill is dealing with what you've got.

3. **Creates natural counterparties** - Alice (73% long) might want to reduce. Bob (28% long) might want to add. Instant liquidity.

4. **Strategic depth** - Opening seconds: "Do I like my position? What's my read? Flatten, double down, or hold?"

5. **Players become their own mandated agents** - The 80% player has a natural "sell-side" inclination. The 20% player is naturally "buy-side."

### Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Range | 20-80% | Never perfectly positioned, never death sentence |
| Per-player variance | Different for each | Creates asymmetry and trading counterparties |
| Visibility during match | Hidden | Information is power; read the tape to infer |
| Post-match reveal | Yes | "You started 73% long on a -2% day. Despite that, +$8k." Adds narrative. |

### Interaction with Match Flow

```
T-60s   LOBBY
        - Historical day selected, NAV established
        - Starting positions generated (hidden from players)
        - Players see: "SPY Arcade - 15 MIN MATCH - NAV: $482.35"

T-30s   PRE-MARKET
        - Players see their starting position for first time
        - "You have: 6,200 shares + $382,000 cash"
        - 30 seconds to process and plan

T=0     BELL RINGS
        - Trading begins with inherited positions
```

### Post-Match Stats

```
MATCH COMPLETE

Your Performance:
  Starting Position:  6,200 shares (64%) + $380,000 cash
  Final Position:     2,100 shares + $812,000 cash
  P&L:               +$23,450 (+2.3%)

Market Context:
  NAV Change:        -1.2% (down day)
  Your Alpha:        +3.5% vs hold

Leaderboard:
  1. Alice    +$41,200  (started 23% long)
  2. YOU      +$23,450  (started 64% long)
  3. Bob      +$12,100  (started 71% long)
```

The reveal adds narrative tension: "I was dealt a tough hand and still made money" or "I had the perfect starting position and blew it."

---

## Implementation Priorities

### Phase 1: Core Mechanics
1. Redemption engine (create/redeem at NAV ± dynamic fee)
2. NAV feed from historical data
3. Starting position generator (20-80% random split)
4. Flat trading fee (0.2¢/share)
5. Basic mandated agent (TWAP only)

### Phase 2: Bot Ecosystem
4. Refactor MarketMaker to use IFV model
5. Add inventory tracking to all MMs
6. Implement VPIN for Nervous MM

### Phase 3: Mandate Sophistication
7. Multiple execution strategies (VWAP, Opportunistic)
8. Urgency curves
9. Volume-matched mandate generation

### Phase 4: Polish
10. UI for redemption mechanism
11. NAV deviation indicator
12. Mandate progress visualization (if we want to surface this)

---

## Design Decisions (Resolved)

1. **Redemption is instant** - Clean, simple, no tick delay.

2. **Redemption fee is dynamic** - Starts low, increases asymptotically with cumulative redemption volume over the match. Prevents infinite arb abuse.

3. **Starting positions are hidden** - Revealed only in post-match stats.

4. **Mandated agents share a volume target** - Total expected volume divided evenly among agents.

5. **Maker/taker fees are flat/low** - Small friction, not a major profit center.

---

## Failure Modes to Watch For

### Liquidity Black Hole
All participants pull back, no one trades, market freezes with huge spread.

**Cause:** Bots widen too much after adverse selection, players refuse to cross.

**Prevention:**
- Noise traders always active (they don't care about spreads)
- At least one MM always quotes (Wide MM is the "dumb money of last resort")
- If spread exceeds threshold, noise traders become more aggressive

### Bot Farming
Player finds repeatable exploit against a specific bot.

**Example:** Panic bot overreacts to every 10¢ move. Player causes tiny uptick, Panic bot dumps, player buys the dump. Repeat forever.

**Prevention:**
- Bots have cooldowns (won't panic twice in 5 seconds)
- Bots have randomness in thresholds
- Bots track P&L and adapt (if losing consistently, change behavior)

### Degenerate Equilibria
Best strategy becomes "do nothing" or "everyone provides liquidity."

**Cause:** Players learn that active trading gets punished by bots.

**Prevention:**
- Match is short (10-15 min) so passive strategies don't accumulate much
- Mandated agents create forced flow that rewards active positioning
- Leaderboard rewards absolute P&L, not risk-adjusted returns

### Corner/Squeeze
Player accumulates huge position and forces infinite squeeze.

**Example:** Player buys everything, bots keep shorting, player keeps buying, price goes to infinity.

**Prevention:**
- Bot inventory limits (stop shorting beyond max position)
- Bot stop-loss logic (cut losses at threshold)
- Player margin requirements (can't buy infinitely)
- Redemption mechanism caps how far price can deviate from NAV

### Collusion
Two players coordinate to farm bots.

**Example:** Alice and Bob alternate pushing price up/down, picking off stale MM quotes.

**Prevention:**
- Adaptive MMs detect "unnatural" volatility and widen massively
- When bots withdraw, players are just trading with each other (zero-sum)
- If bots detect wash-trading patterns, they ignore those trades for signal

---

## Additional Alpha: Player as Market Maker

Players shouldn't only be speculators. Providing liquidity is a viable strategy.

### How It Works
- Player posts limit orders inside the MM spread
- If market bounces, player earns the spread
- Risk: adverse selection (getting filled right before a move)

### Why This Matters
- Creates strategic diversity (not everyone momentum trading)
- Teaches real market-making concepts
- Emergent competition: players compete with bots AND each other for spread

### Example
```
Bot spread: $100.05 / $100.15 (10¢ wide)
Player posts: $100.08 bid, $100.12 offer (4¢ wide)

If random noise trades bounce between bid/ask:
  Player buys at $100.08, sells at $100.12 = 4¢ profit

If informed trader hits player's bid and price drops to $99.90:
  Player bought at $100.08, now underwater 18¢
```

The risk/reward mirrors real market making. Some players will specialize in this.

### Trading Fees (Flat, Low)

Simple flat fee to add friction without being a major factor:

| Action | Fee |
|--------|-----|
| **All trades** | 0.2¢/share |

**Why flat and low:**

1. **Simplicity** - One rule, easy to understand
2. **Not punitive** - Doesn't kill scalping, just adds cost
3. **Prevents pure spam** - Can't profit from 0.1¢ arb anymore
4. **Focus on gameplay** - Fees aren't the interesting part

**Example:**
```
Buy 100 shares @ $100.00:  $10,000.00 + $0.20 fee = $10,000.20
Sell 100 shares @ $100.05: $10,005.00 - $0.20 fee = $10,004.80

Net profit: $4.60 (after $0.40 round-trip fees)
```

**Implementation:**
```go
const TradeFee = 20 // 0.2¢ per share

func CalculateFee(quantity int64) int64 {
    return quantity * TradeFee
}
```

The real friction comes from the dynamic redemption fee, not trading fees. Trading should feel nearly frictionless for normal activity.

---

## Noise Traders: The Camouflage Layer

From Kyle's model: noise traders provide "cover" for informed trading.

### Why They're Critical
Without noise, every trade is informative. MMs would widen to infinity because any trade signals someone knows something.

With noise, MMs can't tell if a buy is:
- Informed player front-running a move
- Random noise bot
- Mandated agent executing TWAP

This uncertainty is what allows the market to function.

### Noise Bot Behavior
```go
type NoiseTader struct {
    avgInterval  time.Duration // How often they trade
    sizeRange    [2]int64      // Min/max order size
    bias         float64       // Slight directional lean (-1 to +1)
}

func (n *NoiseTrader) ShouldTrade() bool {
    // Poisson-ish arrival
    return rand.Float64() < (1.0 / n.avgInterval.Seconds())
}

func (n *NoiseTrader) GenerateOrder() *Order {
    side := Buy
    if rand.Float64() > (0.5 + n.bias/2) {
        side = Sell
    }
    size := rand.Int63n(n.sizeRange[1]-n.sizeRange[0]) + n.sizeRange[0]
    return &Order{Side: side, Quantity: size, Type: Market}
}
```

### Volume Target
Noise traders should generate ~30-50% of match volume. Enough to provide camouflage, not so much that they dominate price discovery.

---

## Reflexive Price Discovery

Key insight from both research sources: **the price should be determined by order flow, not an external oracle.**

### Current Problem
```
PriceGenerator → synthetic price
MarketMaker → quotes around synthetic price
Players → can infer synthetic price, exploit MM
```

### Better Model
```
Order Flow → moves order book mid
MMs → quote around book mid + their IFV adjustments
NAV → provides gravitational anchor via redemption
Players → influence price through their trades
```

The price becomes **reflexive**: player trades move the price, which affects bot behavior, which creates new trading opportunities.

### Implementation
1. MMs weight `bookMid` heavily in their IFV (50%+ weight)
2. NAV provides soft anchor (bots are pulled toward it, but not rigidly)
3. Momentum bots amplify player-initiated moves
4. Mean reversion bots dampen extremes
5. Result: price "breathes" based on collective order flow

---

## Summary

The **Redemption Anchor Model** solves the core design problem:

1. **NAV is public** - everyone knows the "reference price"
2. **Redemption has a fee** - creates a band where market price floats freely
3. **Mandated agents create flow** - natural buying/selling pressure
4. **MMs quote inside the band** - safe as long as they stay within bounds
5. **Players find alpha** - front-running mandates, reading inventory, fading extremes

This creates a market that:
- Feels "alive" (constant order flow from mandates)
- Has discoverable edge (patterns in agent behavior)
- Isn't trivially exploitable (fee prevents infinite arb)
- Creates tension (mandates create deadlines and desperation)
- Rewards skill (reading the tape, timing, risk management)

The order book becomes a battleground where players, MMs, and mandated agents all have conflicting objectives—exactly the "mud wrestling with a box cutter" vision.
