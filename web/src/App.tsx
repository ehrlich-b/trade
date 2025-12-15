import { useState, useEffect, useCallback, useRef } from 'react'
import { BookSnapshot, Trade, Position, WSMessage } from './types'
import OrderBook from './components/OrderBook'
import OrderForm from './components/OrderForm'
import TradeList from './components/TradeList'
import PositionDisplay from './components/PositionDisplay'
import AuthForm from './components/AuthForm'

interface AuthState {
  token: string
  userId: string
  accountId: string
  username: string
}

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

function App() {
  const [auth, setAuth] = useState<AuthState | null>(getStoredAuth)
  const [book, setBook] = useState<BookSnapshot | null>(null)
  const [trades, setTrades] = useState<Trade[]>([])
  const [position, setPosition] = useState<Position>({ quantity: 0, avgPrice: 0, pnl: 0 })
  const [balance, setBalance] = useState(100000000) // $1M default
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)

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
  }

  const fetchAccount = useCallback(async () => {
    if (!auth) return
    try {
      const resp = await fetch('/api/account', {
        headers: { Authorization: `Bearer ${auth.token}` },
      })
      if (resp.ok) {
        const data = await resp.json()
        console.log('[fetchAccount] Response:', data)
        setBalance(data.balance || 100000000)
        const pos = data.positions?.find((p: { symbol: string }) => p.symbol === 'FAKE')
        if (pos) {
          console.log('[fetchAccount] Setting position:', pos)
          setPosition({
            quantity: pos.quantity,
            avgPrice: pos.avg_price,
            pnl: pos.realized_pnl,
          })
        } else {
          console.log('[fetchAccount] No FAKE position, resetting to 0')
          setPosition({ quantity: 0, avgPrice: 0, pnl: 0 })
        }
      } else {
        console.error('[fetchAccount] Failed:', resp.status, await resp.text())
      }
    } catch (err) {
      console.error('[fetchAccount] Error:', err)
    }
  }, [auth])

  const connectWS = useCallback(() => {
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
      setTimeout(connectWS, 2000)
    }

    ws.onerror = (err) => {
      console.error('WebSocket error:', err)
    }

    ws.onmessage = (event) => {
      const msg: WSMessage = JSON.parse(event.data)
      if (msg.type === 'book') {
        setBook(msg.book)
      } else if (msg.type === 'trade') {
        console.log('[WS] Trade received:', msg.trade, 'auth.userId:', auth?.userId)
        setTrades((prev) => [...prev.slice(-49), msg.trade])
        // Refresh position from server if this trade involves us
        if (auth && (msg.trade.buyer_id === auth.userId || msg.trade.seller_id === auth.userId)) {
          console.log('[WS] Trade involves us, fetching account')
          fetchAccount()
        }
      }
    }

    wsRef.current = ws
  }, [auth, fetchAccount])

  useEffect(() => {
    connectWS()
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [connectWS])

  useEffect(() => {
    if (auth) {
      fetchAccount()
    }
  }, [auth, fetchAccount])

  const submitOrder = async (side: 'buy' | 'sell', type: 'limit' | 'market', price: number, quantity: number) => {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (auth) {
      headers['Authorization'] = `Bearer ${auth.token}`
    }

    console.log('[submitOrder] Submitting:', { side, type, price, quantity })
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
      alert('Order failed: ' + text)
    } else {
      const data = await resp.json()
      console.log('[submitOrder] Response:', data)
      // Immediately fetch account to update position (don't rely solely on WebSocket)
      fetchAccount()
    }
  }

  const midPrice = book && book.bids.length > 0 && book.asks.length > 0
    ? (book.bids[0].price + book.asks[0].price) / 2
    : 10000

  if (!auth) {
    return <AuthForm onAuth={handleAuth} />
  }

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>$FAKE</h1>
        <div style={styles.headerRight}>
          <span style={{ color: connected ? '#4ade80' : '#f87171' }}>
            {connected ? '● Connected' : '○ Disconnected'}
          </span>
          <span style={styles.userId}>{auth.username}</span>
          <button onClick={handleLogout} style={styles.logoutButton}>
            Logout
          </button>
        </div>
      </header>

      <main style={styles.main}>
        <div style={styles.leftPanel}>
          <OrderBook book={book} />
        </div>

        <div style={styles.centerPanel}>
          <OrderForm onSubmit={submitOrder} midPrice={midPrice} />
          <PositionDisplay position={position} currentPrice={midPrice} balance={balance} />
        </div>

        <div style={styles.rightPanel}>
          <TradeList trades={trades} userId={auth.userId} />
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
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '16px 24px',
    borderBottom: '1px solid #333',
  },
  title: {
    fontSize: '24px',
    fontWeight: 'bold',
    color: '#fff',
  },
  headerRight: {
    display: 'flex',
    gap: '16px',
    alignItems: 'center',
  },
  userId: {
    color: '#888',
    fontSize: '14px',
  },
  logoutButton: {
    padding: '6px 12px',
    background: '#333',
    border: 'none',
    borderRadius: '4px',
    color: '#888',
    fontSize: '12px',
    cursor: 'pointer',
  },
  main: {
    flex: 1,
    display: 'grid',
    gridTemplateColumns: '300px 1fr 300px',
    gap: '16px',
    padding: '16px',
  },
  leftPanel: {
    background: '#111',
    borderRadius: '8px',
    padding: '16px',
  },
  centerPanel: {
    display: 'flex',
    flexDirection: 'column',
    gap: '16px',
  },
  rightPanel: {
    background: '#111',
    borderRadius: '8px',
    padding: '16px',
  },
}

export default App
