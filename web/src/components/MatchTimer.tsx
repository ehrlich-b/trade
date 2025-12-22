import React from 'react'
import { MatchState } from '../types'

interface MatchTimerProps {
  state: MatchState
  remainingSec: number
  marketTime: string
  duration: number
  progress: number
}

function formatTime(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

export default function MatchTimer({ state, remainingSec, marketTime, duration, progress }: MatchTimerProps) {
  const isFinalCountdown = remainingSec <= 30
  const isWarning = remainingSec <= 90 && remainingSec > 30

  return (
    <div style={styles.container}>
      {/* Match Type Badge */}
      <div style={styles.badge}>
        {duration} MIN MATCH
      </div>

      {/* Big Timer */}
      <div style={{
        ...styles.timer,
        ...(isFinalCountdown ? styles.timerFinal : {}),
        ...(isWarning ? styles.timerWarning : {}),
      }}>
        {state === 'TRADING' ? formatTime(remainingSec) : state}
      </div>

      {/* Market Time */}
      {state === 'TRADING' && (
        <div style={styles.marketTime}>
          {marketTime}
        </div>
      )}

      {/* Progress Bar */}
      {state === 'TRADING' && (
        <div style={styles.progressContainer}>
          <div
            style={{
              ...styles.progressBar,
              width: `${progress * 100}%`,
              backgroundColor: isFinalCountdown ? '#ef4444' : isWarning ? '#f59e0b' : '#22c55e',
            }}
          />
        </div>
      )}

      {/* Final Countdown Alert */}
      {isFinalCountdown && state === 'TRADING' && (
        <div style={styles.finalAlert}>
          FINAL {remainingSec}
        </div>
      )}

      {/* 90 Second Warning */}
      {isWarning && state === 'TRADING' && (
        <div style={styles.warningAlert}>
          90 SECONDS
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
    gap: '8px',
    padding: '16px',
    background: '#0a0a0a',
    borderRadius: '8px',
    border: '1px solid #222',
  },
  badge: {
    fontSize: '11px',
    fontWeight: 'bold',
    color: '#888',
    letterSpacing: '1px',
    padding: '4px 12px',
    background: '#1a1a1a',
    borderRadius: '4px',
  },
  timer: {
    fontSize: '48px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
    color: '#fff',
    letterSpacing: '2px',
  },
  timerWarning: {
    color: '#f59e0b',
  },
  timerFinal: {
    color: '#ef4444',
    animation: 'pulse 0.5s infinite',
  },
  marketTime: {
    fontSize: '14px',
    color: '#666',
    fontFamily: 'monospace',
  },
  progressContainer: {
    width: '100%',
    height: '4px',
    background: '#222',
    borderRadius: '2px',
    overflow: 'hidden',
  },
  progressBar: {
    height: '100%',
    transition: 'width 0.1s linear',
  },
  finalAlert: {
    fontSize: '24px',
    fontWeight: 'bold',
    color: '#ef4444',
    animation: 'pulse 0.5s infinite',
  },
  warningAlert: {
    fontSize: '16px',
    fontWeight: 'bold',
    color: '#f59e0b',
  },
}
