import React from 'react'
import { MatchState, MatchParticipant, MatchResult } from '../types'
import MatchTimer from './MatchTimer'
import MatchLeaderboard from './MatchLeaderboard'
import MatchLobby from './MatchLobby'
import SettlementScreen from './SettlementScreen'

interface MatchOverlayProps {
  state: MatchState
  symbol: string
  duration: number
  nav: number
  participants: MatchParticipant[]
  countdown: number
  remainingSec: number
  marketTime: string
  progress: number
  results: MatchResult[]
  currentUserId: string
  pnlByUser: Record<string, number>
  onJoin: () => void
  isJoined: boolean
  onCloseResults: () => void
}

export default function MatchOverlay({
  state,
  symbol,
  duration,
  nav,
  participants,
  countdown,
  remainingSec,
  marketTime,
  progress,
  results,
  currentUserId,
  pnlByUser,
  onJoin,
  isJoined,
  onCloseResults,
}: MatchOverlayProps) {
  // Show settlement screen as modal
  if (state === 'COMPLETE' && results.length > 0) {
    return (
      <SettlementScreen
        results={results}
        currentUserId={currentUserId}
        finalNav={nav}
        onClose={onCloseResults}
      />
    )
  }

  // Show lobby for LOBBY and PRE_MATCH states
  if (state === 'LOBBY' || state === 'PRE_MATCH') {
    return (
      <div style={styles.lobbyOverlay}>
        <MatchLobby
          symbol={symbol}
          duration={duration}
          participants={participants}
          countdown={state === 'PRE_MATCH' ? countdown : undefined}
          onJoin={onJoin}
          isJoined={isJoined}
          currentUserId={currentUserId}
        />
      </div>
    )
  }

  // During trading, show compact timer and leaderboard overlay
  if (state === 'TRADING') {
    // Convert pnlByUser to leaderboard entries
    const leaderboardEntries = Object.entries(pnlByUser).map(([userId, pnl]) => ({
      userId,
      pnl,
      rank: 0,
      isCurrentUser: userId === currentUserId,
    }))

    return (
      <div style={styles.tradingOverlay}>
        {/* Timer at top */}
        <div style={styles.timerSection}>
          <MatchTimer
            state={state}
            remainingSec={remainingSec}
            marketTime={marketTime}
            duration={duration}
            progress={progress}
          />
        </div>

        {/* Leaderboard at side */}
        <div style={styles.leaderboardSection}>
          <MatchLeaderboard
            entries={leaderboardEntries}
            currentUserId={currentUserId}
          />
        </div>
      </div>
    )
  }

  // Settlement state - waiting for results
  if (state === 'SETTLEMENT') {
    return (
      <div style={styles.settlementOverlay}>
        <div style={styles.settlementBox}>
          <div style={styles.settlementSpinner}>⏳</div>
          <div style={styles.settlementText}>CALCULATING RESULTS...</div>
        </div>
      </div>
    )
  }

  return null
}

const styles: Record<string, React.CSSProperties> = {
  lobbyOverlay: {
    position: 'fixed',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'rgba(0, 0, 0, 0.95)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
  },
  tradingOverlay: {
    position: 'fixed',
    top: 0,
    left: 0,
    right: 0,
    pointerEvents: 'none',
    zIndex: 100,
    display: 'flex',
    justifyContent: 'space-between',
    padding: '12px',
  },
  timerSection: {
    pointerEvents: 'auto',
  },
  leaderboardSection: {
    pointerEvents: 'auto',
    background: '#111',
    borderRadius: '8px',
    padding: '12px',
    border: '1px solid #222',
    maxWidth: '280px',
  },
  settlementOverlay: {
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
  settlementBox: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '16px',
    padding: '48px',
    background: '#111',
    borderRadius: '12px',
    border: '1px solid #333',
  },
  settlementSpinner: {
    fontSize: '48px',
    animation: 'spin 1s linear infinite',
  },
  settlementText: {
    fontSize: '18px',
    color: '#888',
    letterSpacing: '2px',
  },
}
