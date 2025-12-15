import { Position } from '../types'

interface Props {
  position: Position
  currentPrice: number
  balance: number
}

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
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

function PositionDisplay({ position, currentPrice, balance }: Props) {
  const unrealizedPnl = position.quantity * (currentPrice - position.avgPrice)
  const positionValue = position.quantity * currentPrice
  const netWorth = balance + positionValue
  const totalPnl = position.pnl + unrealizedPnl

  return (
    <div style={styles.container}>
      {/* Account Summary - Always Visible */}
      <div style={styles.accountSummary}>
        <div style={styles.summaryItem}>
          <span style={styles.summaryLabel}>Cash</span>
          <span style={styles.summaryValue}>{formatMoney(balance)}</span>
        </div>
        <div style={styles.summaryItem}>
          <span style={styles.summaryLabel}>Position</span>
          <span style={{
            ...styles.summaryValue,
            color: positionValue > 0 ? '#22c55e' : positionValue < 0 ? '#ef4444' : '#888',
          }}>{formatMoney(positionValue)}</span>
        </div>
        <div style={styles.summaryItem}>
          <span style={styles.summaryLabel}>Net Worth</span>
          <span style={{
            ...styles.summaryValue,
            color: netWorth >= 100000000 ? '#22c55e' : '#ef4444',
          }}>{formatMoney(netWorth)}</span>
        </div>
      </div>

      <h2 style={styles.title}>Position</h2>

      <div style={styles.grid}>
        <div style={styles.stat}>
          <div style={styles.label}>Quantity</div>
          <div style={{
            ...styles.value,
            color: position.quantity > 0 ? '#22c55e' : position.quantity < 0 ? '#ef4444' : '#888',
          }}>
            {position.quantity > 0 ? '+' : ''}{position.quantity}
          </div>
        </div>

        <div style={styles.stat}>
          <div style={styles.label}>Avg Price</div>
          <div style={styles.value}>
            {position.avgPrice > 0 ? `$${formatPrice(position.avgPrice)}` : '-'}
          </div>
        </div>

        <div style={styles.stat}>
          <div style={styles.label}>Realized P&L</div>
          <div style={{
            ...styles.value,
            color: position.pnl >= 0 ? '#22c55e' : '#ef4444',
          }}>
            ${formatPrice(position.pnl)}
          </div>
        </div>

        <div style={styles.stat}>
          <div style={styles.label}>Unrealized P&L</div>
          <div style={{
            ...styles.value,
            color: unrealizedPnl >= 0 ? '#22c55e' : '#ef4444',
          }}>
            ${formatPrice(unrealizedPnl)}
          </div>
        </div>
      </div>

      <div style={styles.total}>
        <span style={styles.totalLabel}>Total P&L</span>
        <span style={{
          ...styles.totalValue,
          color: totalPnl >= 0 ? '#22c55e' : '#ef4444',
        }}>
          ${formatPrice(totalPnl)}
        </span>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    background: '#111',
    borderRadius: '8px',
    padding: '16px',
  },
  accountSummary: {
    display: 'flex',
    justifyContent: 'space-between',
    marginBottom: '16px',
    paddingBottom: '16px',
    borderBottom: '1px solid #333',
  },
  summaryItem: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '4px',
  },
  summaryLabel: {
    fontSize: '11px',
    color: '#666',
    textTransform: 'uppercase',
  },
  summaryValue: {
    fontSize: '16px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
    color: '#fff',
  },
  title: {
    fontSize: '14px',
    fontWeight: 'bold',
    marginBottom: '16px',
    color: '#888',
    textTransform: 'uppercase',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '16px',
  },
  stat: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
  },
  label: {
    fontSize: '12px',
    color: '#666',
  },
  value: {
    fontSize: '18px',
    fontFamily: 'monospace',
    color: '#fff',
  },
  total: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginTop: '16px',
    paddingTop: '16px',
    borderTop: '1px solid #333',
  },
  totalLabel: {
    fontSize: '14px',
    color: '#888',
  },
  totalValue: {
    fontSize: '24px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
  },
}

export default PositionDisplay
