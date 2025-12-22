import React from 'react'

interface LeaderboardEntry {
  userId: string
  username?: string
  pnl: number
  rank: number
  isCurrentUser: boolean
}

interface MatchLeaderboardProps {
  entries: LeaderboardEntry[]
  currentUserId: string
}

function formatMoney(cents: number): string {
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

export default function MatchLeaderboard({ entries, currentUserId }: MatchLeaderboardProps) {
  const sortedEntries = [...entries].sort((a, b) => b.pnl - a.pnl)

  return (
    <div style={styles.container}>
      <h3 style={styles.title}>LIVE RANKINGS</h3>
      <div style={styles.list}>
        {sortedEntries.map((entry, index) => {
          const isCurrentUser = entry.userId === currentUserId
          const rank = index + 1

          return (
            <div
              key={entry.userId}
              style={{
                ...styles.entry,
                ...(isCurrentUser ? styles.currentUser : {}),
                ...(rank === 1 ? styles.firstPlace : {}),
              }}
            >
              <div style={styles.rank}>
                {rank === 1 && '🥇'}
                {rank === 2 && '🥈'}
                {rank === 3 && '🥉'}
                {rank > 3 && `#${rank}`}
              </div>
              <div style={styles.username}>
                {entry.username || entry.userId.slice(0, 8)}
                {isCurrentUser && ' (you)'}
              </div>
              <div style={{
                ...styles.pnl,
                color: entry.pnl >= 0 ? '#22c55e' : '#ef4444',
              }}>
                {formatMoney(entry.pnl)}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
  },
  title: {
    fontSize: '12px',
    fontWeight: 'bold',
    color: '#666',
    letterSpacing: '0.5px',
    margin: 0,
  },
  list: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
  },
  entry: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '8px 12px',
    background: '#1a1a1a',
    borderRadius: '4px',
    border: '1px solid transparent',
  },
  currentUser: {
    border: '1px solid #3b82f6',
    background: '#1e3a5f',
  },
  firstPlace: {
    background: '#1a1a0a',
    border: '1px solid #ca8a04',
  },
  rank: {
    fontSize: '14px',
    width: '32px',
    textAlign: 'center',
  },
  username: {
    flex: 1,
    fontSize: '13px',
    color: '#ccc',
    fontFamily: 'monospace',
  },
  pnl: {
    fontSize: '14px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
  },
}
