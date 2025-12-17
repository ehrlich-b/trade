import { useState, useEffect } from 'react'

interface LeaderboardEntry {
  username: string
  total_pnl: number
}

function formatPnL(cents: number): string {
  const dollars = cents / 100
  const sign = dollars >= 0 ? '+' : ''
  if (Math.abs(dollars) >= 1000000) {
    return `${sign}$${(dollars / 1000000).toFixed(2)}M`
  }
  if (Math.abs(dollars) >= 1000) {
    return `${sign}$${(dollars / 1000).toFixed(1)}K`
  }
  return `${sign}$${dollars.toFixed(2)}`
}

function Leaderboard() {
  const [entries, setEntries] = useState<LeaderboardEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchLeaderboard = async () => {
    try {
      const resp = await fetch('/api/leaderboard')
      if (!resp.ok) {
        throw new Error('Failed to load leaderboard')
      }
      const data = await resp.json()
      setEntries(data || [])
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchLeaderboard()
    // Refresh every 30 seconds
    const interval = setInterval(fetchLeaderboard, 30000)
    return () => clearInterval(interval)
  }, [])

  if (loading) {
    return <div style={styles.loading}>Loading leaderboard...</div>
  }

  if (error) {
    return <div style={styles.error}>{error}</div>
  }

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Leaderboard</h2>

      <div style={styles.header}>
        <span>#</span>
        <span>Player</span>
        <span>P&L</span>
      </div>

      <div style={styles.list}>
        {entries.map((entry, index) => (
          <div
            key={entry.username}
            style={{
              ...styles.row,
              backgroundColor: index === 0 ? 'rgba(234, 179, 8, 0.1)' : 'transparent',
            }}
          >
            <span style={styles.rank}>
              {index === 0 ? '🥇' : index === 1 ? '🥈' : index === 2 ? '🥉' : index + 1}
            </span>
            <span style={styles.username}>{entry.username}</span>
            <span
              style={{
                ...styles.pnl,
                color: entry.total_pnl >= 0 ? '#22c55e' : '#ef4444',
              }}
            >
              {formatPnL(entry.total_pnl)}
            </span>
          </div>
        ))}
        {entries.length === 0 && (
          <div style={styles.empty}>No players yet</div>
        )}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    height: '100%',
  },
  loading: {
    color: '#888',
    textAlign: 'center',
    padding: '20px',
  },
  error: {
    color: '#ef4444',
    textAlign: 'center',
    padding: '20px',
  },
  title: {
    fontSize: '14px',
    fontWeight: 'bold',
    marginBottom: '12px',
    color: '#888',
    textTransform: 'uppercase',
  },
  header: {
    display: 'grid',
    gridTemplateColumns: '30px 1fr 80px',
    fontSize: '12px',
    color: '#666',
    marginBottom: '8px',
    padding: '0 4px',
  },
  list: {
    flex: 1,
    overflow: 'auto',
  },
  row: {
    display: 'grid',
    gridTemplateColumns: '30px 1fr 80px',
    padding: '6px 4px',
    fontSize: '13px',
    borderRadius: '4px',
    alignItems: 'center',
  },
  rank: {
    color: '#888',
    textAlign: 'center',
  },
  username: {
    color: '#fff',
    fontWeight: 500,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  pnl: {
    fontFamily: 'monospace',
    textAlign: 'right',
  },
  empty: {
    color: '#666',
    textAlign: 'center',
    padding: '20px',
  },
}

export default Leaderboard
