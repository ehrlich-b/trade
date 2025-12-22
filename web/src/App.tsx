import { useState, useEffect, useRef } from 'react'
import { BookSnapshot, Trade, Position, Order, BarData } from './types'
import OrderBook from './components/OrderBook'
import OrderForm from './components/OrderForm'
import TradeList from './components/TradeList'
import AuthForm from './components/AuthForm'
import Leaderboard from './components/Leaderboard'
import OpenOrders from './components/OpenOrders'
import CandlestickChart from './components/CandlestickChart'

interface AuthState {
  token: string
  userId: string
  accountId: string
  username: string
}

const SYMBOLS = ['SPY'] // Arcade mode uses SPY

function getStoredAuth(): AuthState | null {
  const stored = localStorage.getItem('auth')
  if (stored) {
    try {
      return JSON.parse(stored)
    } catch {
      return null
    }
  }
  return null
}

function formatMoney(cents: number): string {
  const dollars = cents / 100
  if (Math.abs(dollars) >= 1000000) {
    return `$${(dollars / 1000000).toFixed(2)}M`
  }
  if (Math.abs(dollars) >= 1000) {
    return `$${(dollars / 1000).toFixed(1)}K`
  }
  return `$${dollars.toFixed(2)}`
}

function App() {
  const [auth, setAuth] = useState<AuthState | null>(getStoredAuth)
  const [symbol, setSymbol] = useState('SPY')
  const [book, setBook] = useState<BookSnapshot | null>(null)
  const [trades, setTrades] = useState<Trade[]>([])
  const [position, setPosition] = useState<Position>({ quantity: 0, avgPrice: 0, pnl: 0 })
  const [balance, setBalance] = useState(100000000) // $1M default
  const [connected, setConnected] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [showLeaderboard, setShowLeaderboard] = useState(false)
  const [openOrders, setOpenOrders] = useState<Order[]>([])
  const [cancelling, setCancelling] = useState<Record<string, boolean>>({})
  const [bars, setBars] = useState<BarData[]>([])
  const [currentBar, setCurrentBar] = useState(0)
  const [marketTime, setMarketTime] = useState('9:30 AM')
  const wsRef = useRef<WebSocket | null>(null)

  // Use ref for auth to avoid stale closures in callbacks
  const authRef = useRef(auth)
  authRef.current = auth

  const handleAuth = (token: string, userId: string, accountId: string, username: string) => {
    const authState = { token, userId, accountId, username }
    setAuth(authState)
    localStorage.setItem('auth', JSON.stringify(authState))
  }

  const handleLogout = () => {
    setAuth(null)
    localStorage.removeItem('auth')
    setPosition({ quantity: 0, avgPrice: 0, pnl: 0 })
    setBalance(100000000)
    setOpenOrders([])
  }

  // Fetch account data - uses authRef to always get current auth (avoids stale closures)
  const fetchAccount = async () => {
    const currentAuth = authRef.current
    if (!currentAuth) return
    try {
      const resp = await fetch('/api/account', {
        headers: { Authorization: `Bearer ${currentAuth.token}` },
      })
      if (resp.status === 401) {
        // Token expired or invalid - clear auth and force re-login
        handleLogout()
        return
      }
      if (resp.ok) {
        const data = await resp.json()
        setBalance(data.balance || 100000000)
        const pos = data.positions?.find((p: { symbol: string }) => p.symbol === 'SPY')
        if (pos) {
          setPosition({
            quantity: pos.quantity,
            avgPrice: pos.avg_price,
            pnl: pos.realized_pnl,
          })
        } else {
          setPosition({ quantity: 0, avgPrice: 0, pnl: 0 })
        }
      }
    } catch (err) {
      console.error('Failed to fetch account:', err)
    }
  }

  // Fetch open orders
  const fetchOrders = async () => {
    const currentAuth = authRef.current
    if (!currentAuth) return
    try {
      const resp = await fetch('/api/orders', {
        headers: { Authorization: `Bearer ${currentAuth.token}` },
      })
      if (resp.status === 401) {
        // Token expired or invalid - clear auth and force re-login
        handleLogout()
        return
      }
      if (resp.ok) {
        const data = await resp.json()
        setOpenOrders(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch orders:', err)
    }
  }

  // Cancel an order
  const cancelOrder = async (orderId: string) => {
    const currentAuth = authRef.current
    if (!currentAuth) return
    setCancelling((prev) => ({ ...prev, [orderId]: true }))
    try {
      const resp = await fetch(`/api/orders/${orderId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${currentAuth.token}` },
      })
      if (resp.ok) {
        setOpenOrders((prev) => prev.filter((o) => o.id !== orderId))
      } else {
        const text = await resp.text()
        console.error('Cancel failed:', text)
      }
    } catch (err) {
      console.error('Failed to cancel order:', err)
    } finally {
      setCancelling((prev) => ({ ...prev, [orderId]: false }))
    }
  }

  // WebSocket connection - only set up once, uses refs for current values
  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      console.log('WebSocket connected')
      setConnected(true)
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
      setConnected(false)
    }

    ws.onerror = (err) => {
      console.error('WebSocket error:', err)
    }

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      if (msg.type === 'book') {
        setBook(msg.book)
      } else if (msg.type === 'trade') {
        const currentAuth = authRef.current
        setTrades((prev) => [...prev.slice(-49), msg.trade])
        // Refresh position and orders if trade involves us
        if (currentAuth && (msg.trade.buyer_id === currentAuth.userId || msg.trade.seller_id === currentAuth.userId)) {
          fetchAccount()
          fetchOrders()
        }
      } else if (msg.type === 'match_state') {
        // Update bars and market time from match state
        if (msg.bars && Array.isArray(msg.bars)) {
          setBars(msg.bars)
        }
        if (typeof msg.current_bar === 'number') {
          setCurrentBar(msg.current_bar)
        }
        if (msg.market_time) {
          setMarketTime(msg.market_time)
        }
      }
    }

    wsRef.current = ws
    return () => ws.close()
  }, []) // Empty deps - only run once

  // Fetch account and orders when auth changes
  useEffect(() => {
    if (auth) {
      fetchAccount()
      fetchOrders()
    }
  }, [auth])

  const submitOrder = async (side: 'buy' | 'sell', type: 'limit' | 'market', price: number, quantity: number) => {
    if (submitting) return

    setSubmitting(true)
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (auth) {
      headers['Authorization'] = `Bearer ${auth.token}`
    }

    try {
      const resp = await fetch('/api/orders', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          user_id: auth?.userId || 'anonymous',
          side,
          type,
          price: type === 'limit' ? price : 0,
          quantity,
        }),
      })
      if (!resp.ok) {
        const text = await resp.text()
        console.error('Order failed:', text)
      } else {
        // Parse response but don't block on it
        resp.json().catch(() => {})
        // Refresh account and orders
        fetchAccount()
        fetchOrders()
      }
    } catch (err) {
      console.error('Order submission error:', err)
    } finally {
      setSubmitting(false)
    }
  }

  const midPrice = book && book.bids.length > 0 && book.asks.length > 0
    ? (book.bids[0].price + book.asks[0].price) / 2
    : 10000

  const lastPrice = trades.length > 0 ? trades[trades.length - 1].price : null
  const spread = book && book.bids.length > 0 && book.asks.length > 0
    ? book.asks[0].price - book.bids[0].price
    : 0

  // Calculate account metrics
  const positionValue = position.quantity * midPrice
  const netWorth = balance + positionValue
  const unrealizedPnl = position.quantity * (midPrice - position.avgPrice)
  const totalPnl = position.pnl + unrealizedPnl

  if (!auth) {
    return <AuthForm onAuth={handleAuth} />
  }

  return (
    <div style={styles.container}>
      {/* Header - Symbol, Prices, Account Summary */}
      <header style={styles.header}>
        <div style={styles.headerLeft}>
          <select
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            style={styles.symbolSelect}
          >
            {SYMBOLS.map((s) => (
              <option key={s} value={s}>${s}</option>
            ))}
          </select>
          <div style={styles.priceGroup}>
            <div style={styles.priceItem}>
              <span style={styles.priceLabel}>MARKET TIME</span>
              <span style={styles.priceValue}>{marketTime}</span>
            </div>
            <div style={styles.priceItem}>
              <span style={styles.priceLabel}>LAST</span>
              <span style={styles.priceValue}>
                {lastPrice !== null ? `$${(lastPrice / 100).toFixed(2)}` : '—'}
              </span>
            </div>
            <div style={styles.priceItem}>
              <span style={styles.priceLabel}>MID</span>
              <span style={styles.priceValue}>${(midPrice / 100).toFixed(2)}</span>
            </div>
            <div style={styles.priceItem}>
              <span style={styles.priceLabel}>SPREAD</span>
              <span style={styles.spreadValue}>${(spread / 100).toFixed(2)}</span>
            </div>
          </div>
        </div>

        <div style={styles.accountSummary}>
          <div style={styles.accountItem}>
            <span style={styles.accountLabel}>CASH</span>
            <span style={styles.accountValue}>{formatMoney(balance)}</span>
          </div>
          <div style={styles.accountItem}>
            <span style={styles.accountLabel}>POSITION</span>
            <span style={{
              ...styles.accountValue,
              color: position.quantity > 0 ? '#22c55e' : position.quantity < 0 ? '#ef4444' : '#888',
            }}>
              {position.quantity !== 0 ? `${position.quantity > 0 ? '+' : ''}${position.quantity}` : '—'}
            </span>
          </div>
          <div style={styles.accountItem}>
            <span style={styles.accountLabel}>P&L</span>
            <span style={{
              ...styles.accountValue,
              color: totalPnl >= 0 ? '#22c55e' : '#ef4444',
            }}>
              {formatMoney(totalPnl)}
            </span>
          </div>
          <div style={styles.accountItem}>
            <span style={styles.accountLabel}>NET WORTH</span>
            <span style={{
              ...styles.accountValue,
              color: netWorth >= 100000000 ? '#22c55e' : '#ef4444',
            }}>
              {formatMoney(netWorth)}
            </span>
          </div>
        </div>

        <div style={styles.headerRight}>
          <span style={{ color: connected ? '#4ade80' : '#f87171', fontSize: '12px' }}>
            {connected ? '● LIVE' : '○ OFFLINE'}
          </span>
          <span style={styles.username}>{auth.username}</span>
          <button onClick={handleLogout} style={styles.logoutButton}>
            Logout
          </button>
        </div>
      </header>

      {/* Main Trading Area */}
      <main style={styles.main}>
        {/* Left: Order Book */}
        <div style={styles.panel}>
          <OrderBook book={book} userOrders={openOrders} />
        </div>

        {/* Center: Chart + Order Form + Open Orders + Position Details */}
        <div style={styles.centerPanel}>
          <div style={styles.chartPanel}>
            <CandlestickChart bars={bars} currentBar={currentBar} />
          </div>
          <div style={styles.orderRow}>
            <div style={styles.panel}>
              <OrderForm onSubmit={submitOrder} midPrice={midPrice} submitting={submitting} />
            </div>
            <div style={styles.panel}>
              <OpenOrders orders={openOrders} onCancel={cancelOrder} cancelling={cancelling} />
            </div>
          </div>
          <div style={styles.panel}>
            <h3 style={styles.panelTitle}>POSITION DETAILS</h3>
            <div style={styles.positionGrid}>
              <div style={styles.positionItem}>
                <span style={styles.positionLabel}>Quantity</span>
                <span style={{
                  ...styles.positionValue,
                  color: position.quantity > 0 ? '#22c55e' : position.quantity < 0 ? '#ef4444' : '#888',
                }}>
                  {position.quantity !== 0 ? `${position.quantity > 0 ? '+' : ''}${position.quantity}` : '0'}
                </span>
              </div>
              <div style={styles.positionItem}>
                <span style={styles.positionLabel}>Avg Price</span>
                <span style={styles.positionValue}>
                  {position.avgPrice > 0 ? `$${(position.avgPrice / 100).toFixed(2)}` : '—'}
                </span>
              </div>
              <div style={styles.positionItem}>
                <span style={styles.positionLabel}>Realized P&L</span>
                <span style={{
                  ...styles.positionValue,
                  color: position.pnl >= 0 ? '#22c55e' : '#ef4444',
                }}>
                  {formatMoney(position.pnl)}
                </span>
              </div>
              <div style={styles.positionItem}>
                <span style={styles.positionLabel}>Unrealized P&L</span>
                <span style={{
                  ...styles.positionValue,
                  color: unrealizedPnl >= 0 ? '#22c55e' : '#ef4444',
                }}>
                  {formatMoney(unrealizedPnl)}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Right: Trades + Leaderboard Toggle */}
        <div style={styles.rightColumn}>
          <div style={styles.tabBar}>
            <button
              onClick={() => setShowLeaderboard(false)}
              style={{
                ...styles.tabButton,
                ...(showLeaderboard ? {} : styles.tabButtonActive),
              }}
            >
              TRADES
            </button>
            <button
              onClick={() => setShowLeaderboard(true)}
              style={{
                ...styles.tabButton,
                ...(showLeaderboard ? styles.tabButtonActive : {}),
              }}
            >
              LEADERBOARD
            </button>
          </div>
          <div style={styles.panel}>
            {showLeaderboard ? (
              <Leaderboard />
            ) : (
              <TradeList trades={trades} userId={auth.userId} />
            )}
          </div>
        </div>
      </main>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    flexDirection: 'column',
    background: '#0a0a0a',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '12px 20px',
    borderBottom: '1px solid #222',
    background: '#111',
  },
  headerLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: '24px',
  },
  symbolSelect: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '4px',
    color: '#fff',
    fontSize: '18px',
    fontWeight: 'bold',
    padding: '8px 12px',
    cursor: 'pointer',
  },
  priceGroup: {
    display: 'flex',
    gap: '20px',
  },
  priceItem: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '2px',
  },
  priceLabel: {
    fontSize: '10px',
    color: '#666',
    letterSpacing: '0.5px',
  },
  priceValue: {
    fontSize: '16px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
    color: '#fff',
  },
  spreadValue: {
    fontSize: '16px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
    color: '#666',
  },
  accountSummary: {
    display: 'flex',
    gap: '32px',
  },
  accountItem: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '2px',
  },
  accountLabel: {
    fontSize: '10px',
    color: '#666',
    letterSpacing: '0.5px',
  },
  accountValue: {
    fontSize: '16px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
    color: '#fff',
  },
  headerRight: {
    display: 'flex',
    gap: '16px',
    alignItems: 'center',
  },
  username: {
    color: '#888',
    fontSize: '13px',
    fontFamily: 'monospace',
  },
  logoutButton: {
    padding: '6px 12px',
    background: '#222',
    border: '1px solid #333',
    borderRadius: '4px',
    color: '#888',
    fontSize: '11px',
    cursor: 'pointer',
  },
  main: {
    flex: 1,
    display: 'grid',
    gridTemplateColumns: '280px 1fr 280px',
    gap: '12px',
    padding: '12px',
  },
  panel: {
    background: '#111',
    borderRadius: '6px',
    padding: '12px',
    border: '1px solid #222',
  },
  centerPanel: {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
  },
  chartPanel: {
    background: '#111',
    borderRadius: '6px',
    border: '1px solid #222',
    overflow: 'hidden',
    flex: 1,
    minHeight: '320px',
  },
  orderRow: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '12px',
  },
  panelTitle: {
    fontSize: '12px',
    fontWeight: 'bold',
    color: '#666',
    marginBottom: '12px',
    letterSpacing: '0.5px',
  },
  positionGrid: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '16px',
  },
  positionItem: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
  },
  positionLabel: {
    fontSize: '11px',
    color: '#666',
  },
  positionValue: {
    fontSize: '18px',
    fontFamily: 'monospace',
    color: '#fff',
  },
  rightColumn: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0',
  },
  tabBar: {
    display: 'flex',
    marginBottom: '-1px',
  },
  tabButton: {
    flex: 1,
    padding: '8px 12px',
    background: '#0a0a0a',
    border: '1px solid #222',
    borderBottom: 'none',
    borderRadius: '6px 6px 0 0',
    color: '#666',
    fontSize: '11px',
    fontWeight: 'bold',
    cursor: 'pointer',
    letterSpacing: '0.5px',
  },
  tabButtonActive: {
    background: '#111',
    color: '#fff',
    borderColor: '#222',
    borderBottomColor: '#111',
  },
}

export default App
