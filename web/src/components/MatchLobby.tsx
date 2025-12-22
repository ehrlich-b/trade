import React from 'react'
import { MatchParticipant } from '../types'

interface MatchLobbyProps {
  symbol: string
  duration: number
  participants: MatchParticipant[]
  countdown?: number
  onJoin: () => void
  isJoined: boolean
  currentUserId: string
}

export default function MatchLobby({
  symbol,
  duration,
  participants,
  countdown,
  onJoin,
  isJoined,
  currentUserId,
}: MatchLobbyProps) {
  return (
    <div style={styles.container}>
      {/* Spinning Effect */}
      <div style={styles.spinnerSection}>
        <div style={styles.spinnerLabel}>SELECTING TRADING DAY...</div>
        <div style={styles.spinner}>
          <div style={styles.symbol}>${symbol}</div>
          <div style={styles.arcadeText}>ARCADE</div>
        </div>
      </div>

      {/* Match Info */}
      <div style={styles.matchInfo}>
        <div style={styles.durationBadge}>{duration} MIN MATCH</div>
        {countdown !== undefined && countdown > 0 && (
          <div style={styles.countdown}>
            Starting in {countdown}...
          </div>
        )}
      </div>

      {/* Participants */}
      <div style={styles.participantsSection}>
        <div style={styles.participantsHeader}>
          <span style={styles.participantsTitle}>PLAYERS</span>
          <span style={styles.participantsCount}>{participants.length}</span>
        </div>
        <div style={styles.participantsList}>
          {participants.map((p) => (
            <div
              key={p.user_id}
              style={{
                ...styles.participant,
                ...(p.user_id === currentUserId ? styles.currentUserParticipant : {}),
              }}
            >
              {p.user_id.slice(0, 12)}
              {p.user_id === currentUserId && ' (you)'}
            </div>
          ))}
          {participants.length === 0 && (
            <div style={styles.emptyParticipants}>
              Waiting for players...
            </div>
          )}
        </div>
      </div>

      {/* Join Button */}
      {!isJoined && (
        <button onClick={onJoin} style={styles.joinButton}>
          JOIN MATCH
        </button>
      )}
      {isJoined && (
        <div style={styles.joinedBadge}>
          READY
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '24px',
    padding: '32px',
    background: '#111',
    borderRadius: '12px',
    border: '1px solid #333',
    maxWidth: '400px',
    margin: '0 auto',
  },
  spinnerSection: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '12px',
  },
  spinnerLabel: {
    fontSize: '11px',
    color: '#666',
    letterSpacing: '1px',
  },
  spinner: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '4px',
  },
  symbol: {
    fontSize: '48px',
    fontWeight: 'bold',
    color: '#fff',
    fontFamily: 'monospace',
  },
  arcadeText: {
    fontSize: '14px',
    color: '#f59e0b',
    letterSpacing: '4px',
    fontWeight: 'bold',
  },
  matchInfo: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '8px',
  },
  durationBadge: {
    fontSize: '12px',
    fontWeight: 'bold',
    color: '#888',
    letterSpacing: '1px',
    padding: '6px 16px',
    background: '#1a1a1a',
    borderRadius: '4px',
  },
  countdown: {
    fontSize: '18px',
    fontWeight: 'bold',
    color: '#22c55e',
    fontFamily: 'monospace',
  },
  participantsSection: {
    width: '100%',
  },
  participantsHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '8px',
  },
  participantsTitle: {
    fontSize: '11px',
    color: '#666',
    letterSpacing: '0.5px',
  },
  participantsCount: {
    fontSize: '11px',
    color: '#888',
    fontFamily: 'monospace',
  },
  participantsList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
    maxHeight: '150px',
    overflow: 'auto',
  },
  participant: {
    padding: '8px 12px',
    background: '#1a1a1a',
    borderRadius: '4px',
    fontSize: '13px',
    color: '#ccc',
    fontFamily: 'monospace',
  },
  currentUserParticipant: {
    background: '#1e3a5f',
    border: '1px solid #3b82f6',
  },
  emptyParticipants: {
    padding: '12px',
    textAlign: 'center',
    color: '#666',
    fontSize: '13px',
  },
  joinButton: {
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
  joinedBadge: {
    padding: '14px 32px',
    background: '#1a1a1a',
    borderRadius: '6px',
    color: '#22c55e',
    fontSize: '14px',
    fontWeight: 'bold',
    letterSpacing: '1px',
    border: '1px solid #22c55e',
  },
}
