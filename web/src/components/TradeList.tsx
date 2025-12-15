import { Trade } from '../types'

interface Props {
  trades: Trade[]
  userId: string
}

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString()
}

function TradeList({ trades, userId }: Props) {
  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Recent Trades</h2>

      <div style={styles.header}>
        <span>Price</span>
        <span>Size</span>
        <span>Time</span>
      </div>

      <div style={styles.list}>
        {[...trades].reverse().map((trade) => {
          const isYours = trade.buyer_id === userId || trade.seller_id === userId
          return (
            <div
              key={trade.id}
              style={{
                ...styles.row,
                backgroundColor: isYours ? 'rgba(59, 130, 246, 0.1)' : 'transparent',
              }}
            >
              <span style={styles.price}>${formatPrice(trade.price)}</span>
              <span style={styles.qty}>{trade.quantity}</span>
              <span style={styles.time}>{formatTime(trade.timestamp)}</span>
            </div>
          )
        })}
        {trades.length === 0 && (
          <div style={styles.empty}>No trades yet</div>
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
  title: {
    fontSize: '14px',
    fontWeight: 'bold',
    marginBottom: '12px',
    color: '#888',
    textTransform: 'uppercase',
  },
  header: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr 1fr',
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
    gridTemplateColumns: '1fr 1fr 1fr',
    padding: '4px',
    fontSize: '13px',
    fontFamily: 'monospace',
    borderRadius: '2px',
  },
  price: {
    color: '#fff',
  },
  qty: {
    color: '#999',
  },
  time: {
    color: '#666',
    fontSize: '11px',
  },
  empty: {
    color: '#666',
    textAlign: 'center',
    padding: '20px',
  },
}

export default TradeList
