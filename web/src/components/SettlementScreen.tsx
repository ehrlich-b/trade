import React from 'react'
import { MatchResult } from '../types'

interface SettlementScreenProps {
  results: MatchResult[]
  currentUserId: string
  finalNav: number
  onClose: () => void
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

function formatPercent(value: number, base: number): string {
  if (base === 0) return '+0.0%'
  const pct = ((value / base) * 100)
  const sign = pct >= 0 ? '+' : ''
  return `${sign}${pct.toFixed(1)}%`
}

export default function SettlementScreen({ results, currentUserId, finalNav, onClose }: SettlementScreenProps) {
  const myResult = results.find(r => r.user_id === currentUserId)
  const sortedResults = [...results].sort((a, b) => b.pnl - a.pnl)

  return (
    <div style={styles.overlay}>
      <div style={styles.modal}>
        {/* Header */}
        <div style={styles.header}>
          <h2 style={styles.title}>MATCH COMPLETE</h2>
          <button onClick={onClose} style={styles.closeButton}>×</button>
        </div>

        {/* Your Performance */}
        {myResult && (
          <div style={styles.yourPerformance}>
            <div style={styles.rankBadge}>
              {myResult.rank === 1 && '🥇'}
              {myResult.rank === 2 && '🥈'}
              {myResult.rank === 3 && '🥉'}
              {myResult.rank > 3 && `#${myResult.rank}`}
            </div>

            <div style={styles.pnlSection}>
              <div style={styles.pnlLabel}>YOUR P&L</div>
              <div style={{
                ...styles.pnlValue,
                color: myResult.pnl >= 0 ? '#22c55e' : '#ef4444',
              }}>
                {formatMoney(myResult.pnl)}
              </div>
              <div style={{
                ...styles.pnlPercent,
                color: myResult.pnl >= 0 ? '#22c55e' : '#ef4444',
              }}>
                {formatPercent(myResult.pnl, myResult.start_value)}
              </div>
            </div>

            <div style={styles.statsGrid}>
              <div style={styles.statItem}>
                <span style={styles.statLabel}>Starting Position</span>
                <span style={styles.statValue}>
                  {myResult.start_shares} shares ({Math.round(myResult.start_shares * finalNav / myResult.start_value * 100)}%)
                </span>
              </div>
              <div style={styles.statItem}>
                <span style={styles.statLabel}>Final Position</span>
                <span style={styles.statValue}>
                  {myResult.final_shares} shares
                </span>
              </div>
              <div style={styles.statItem}>
                <span style={styles.statLabel}>Starting Value</span>
                <span style={styles.statValue}>{formatMoney(myResult.start_value)}</span>
              </div>
              <div style={styles.statItem}>
                <span style={styles.statLabel}>Final Value</span>
                <span style={styles.statValue}>{formatMoney(myResult.final_value)}</span>
              </div>
            </div>
          </div>
        )}

        {/* Full Leaderboard */}
        <div style={styles.leaderboard}>
          <h3 style={styles.sectionTitle}>FINAL STANDINGS</h3>
          <div style={styles.leaderboardList}>
            {sortedResults.map((result, index) => {
              const isCurrentUser = result.user_id === currentUserId
              const rank = index + 1

              return (
                <div
                  key={result.user_id}
                  style={{
                    ...styles.leaderboardEntry,
                    ...(isCurrentUser ? styles.currentUserEntry : {}),
                  }}
                >
                  <div style={styles.entryRank}>
                    {rank === 1 && '🥇'}
                    {rank === 2 && '🥈'}
                    {rank === 3 && '🥉'}
                    {rank > 3 && `#${rank}`}
                  </div>
                  <div style={styles.entryUser}>
                    {result.user_id.slice(0, 12)}
                    {isCurrentUser && ' (you)'}
                  </div>
                  <div style={{
                    ...styles.entryPnl,
                    color: result.pnl >= 0 ? '#22c55e' : '#ef4444',
                  }}>
                    {formatMoney(result.pnl)}
                  </div>
                  <div style={styles.entryStarting}>
                    Started {Math.round(result.start_shares * finalNav / result.start_value * 100)}% long
                  </div>
                </div>
              )
            })}
          </div>
        </div>

        {/* Next Match Button */}
        <button onClick={onClose} style={styles.nextButton}>
          NEXT MATCH
        </button>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'rgba(0, 0, 0, 0.9)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
  },
  modal: {
    background: '#111',
    borderRadius: '12px',
    border: '1px solid #333',
    padding: '24px',
    maxWidth: '600px',
    width: '100%',
    maxHeight: '90vh',
    overflow: 'auto',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '24px',
  },
  title: {
    fontSize: '24px',
    fontWeight: 'bold',
    color: '#fff',
    margin: 0,
  },
  closeButton: {
    background: 'none',
    border: 'none',
    color: '#666',
    fontSize: '24px',
    cursor: 'pointer',
  },
  yourPerformance: {
    background: '#1a1a1a',
    borderRadius: '8px',
    padding: '20px',
    marginBottom: '24px',
    textAlign: 'center',
  },
  rankBadge: {
    fontSize: '48px',
    marginBottom: '16px',
  },
  pnlSection: {
    marginBottom: '20px',
  },
  pnlLabel: {
    fontSize: '12px',
    color: '#666',
    letterSpacing: '1px',
    marginBottom: '4px',
  },
  pnlValue: {
    fontSize: '36px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
  },
  pnlPercent: {
    fontSize: '18px',
    fontFamily: 'monospace',
  },
  statsGrid: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '12px',
    textAlign: 'left',
  },
  statItem: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
  },
  statLabel: {
    fontSize: '11px',
    color: '#666',
  },
  statValue: {
    fontSize: '14px',
    color: '#ccc',
    fontFamily: 'monospace',
  },
  leaderboard: {
    marginBottom: '24px',
  },
  sectionTitle: {
    fontSize: '12px',
    fontWeight: 'bold',
    color: '#666',
    letterSpacing: '0.5px',
    marginBottom: '12px',
  },
  leaderboardList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
  },
  leaderboardEntry: {
    display: 'grid',
    gridTemplateColumns: '40px 1fr 100px 120px',
    alignItems: 'center',
    gap: '8px',
    padding: '10px 12px',
    background: '#1a1a1a',
    borderRadius: '4px',
    border: '1px solid transparent',
  },
  currentUserEntry: {
    border: '1px solid #3b82f6',
    background: '#1e3a5f',
  },
  entryRank: {
    fontSize: '14px',
    textAlign: 'center',
  },
  entryUser: {
    fontSize: '13px',
    color: '#ccc',
    fontFamily: 'monospace',
  },
  entryPnl: {
    fontSize: '14px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
    textAlign: 'right',
  },
  entryStarting: {
    fontSize: '11px',
    color: '#666',
    textAlign: 'right',
  },
  nextButton: {
    width: '100%',
    padding: '14px',
    background: '#22c55e',
    border: 'none',
    borderRadius: '6px',
    color: '#000',
    fontSize: '14px',
    fontWeight: 'bold',
    cursor: 'pointer',
    letterSpacing: '1px',
  },
}
