import { BookSnapshot } from '../types'

interface Props {
  book: BookSnapshot | null
}

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function OrderBook({ book }: Props) {
  if (!book) {
    return <div style={styles.loading}>Loading order book...</div>
  }

  const maxQty = Math.max(
    ...book.bids.map((b) => b.quantity),
    ...book.asks.map((a) => a.quantity),
    1
  )

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Order Book</h2>

      <div style={styles.header}>
        <span>Price</span>
        <span>Size</span>
      </div>

      <div style={styles.asks}>
        {[...book.asks].reverse().slice(0, 10).map((level, i) => (
          <div key={`ask-${i}`} style={styles.row}>
            <div
              style={{
                ...styles.depthBar,
                ...styles.askBar,
                width: `${(level.quantity / maxQty) * 100}%`,
              }}
            />
            <span style={styles.askPrice}>${formatPrice(level.price)}</span>
            <span style={styles.qty}>{level.quantity}</span>
          </div>
        ))}
      </div>

      <div style={styles.spread}>
        {book.bids.length > 0 && book.asks.length > 0 && (
          <>Spread: ${formatPrice(book.asks[0].price - book.bids[0].price)}</>
        )}
      </div>

      <div style={styles.bids}>
        {book.bids.slice(0, 10).map((level, i) => (
          <div key={`bid-${i}`} style={styles.row}>
            <div
              style={{
                ...styles.depthBar,
                ...styles.bidBar,
                width: `${(level.quantity / maxQty) * 100}%`,
              }}
            />
            <span style={styles.bidPrice}>${formatPrice(level.price)}</span>
            <span style={styles.qty}>{level.quantity}</span>
          </div>
        ))}
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
  title: {
    fontSize: '14px',
    fontWeight: 'bold',
    marginBottom: '12px',
    color: '#888',
    textTransform: 'uppercase',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    fontSize: '12px',
    color: '#666',
    marginBottom: '8px',
    padding: '0 4px',
  },
  asks: {
    display: 'flex',
    flexDirection: 'column',
  },
  bids: {
    display: 'flex',
    flexDirection: 'column',
  },
  spread: {
    textAlign: 'center',
    padding: '8px',
    fontSize: '12px',
    color: '#666',
    borderTop: '1px solid #333',
    borderBottom: '1px solid #333',
    margin: '4px 0',
  },
  row: {
    display: 'flex',
    justifyContent: 'space-between',
    padding: '2px 4px',
    position: 'relative',
    fontSize: '13px',
    fontFamily: 'monospace',
  },
  depthBar: {
    position: 'absolute',
    right: 0,
    top: 0,
    bottom: 0,
    opacity: 0.2,
  },
  askBar: {
    backgroundColor: '#ef4444',
  },
  bidBar: {
    backgroundColor: '#22c55e',
  },
  askPrice: {
    color: '#ef4444',
    zIndex: 1,
  },
  bidPrice: {
    color: '#22c55e',
    zIndex: 1,
  },
  qty: {
    color: '#999',
    zIndex: 1,
  },
}

export default OrderBook
