import { Order } from '../types'

interface Props {
  orders: Order[]
  onCancel: (orderId: string) => Promise<void>
  cancelling: Record<string, boolean>
}

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString()
}

function OpenOrders({ orders, onCancel, cancelling }: Props) {
  // Sort by timestamp descending (newest first)
  const sortedOrders = [...orders].sort(
    (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
  )

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Open Orders</h2>

      <div style={styles.header}>
        <span>Side</span>
        <span>Price</span>
        <span>Qty</span>
        <span>Filled</span>
        <span></span>
      </div>

      <div style={styles.list}>
        {sortedOrders.map((order) => {
          const remaining = order.quantity - order.filled
          const isBuy = order.side === 0
          return (
            <div key={order.id} style={styles.row}>
              <span style={{ color: isBuy ? '#22c55e' : '#ef4444' }}>
                {isBuy ? 'BUY' : 'SELL'}
              </span>
              <span style={styles.price}>${formatPrice(order.price)}</span>
              <span style={styles.qty}>{remaining}/{order.quantity}</span>
              <span style={styles.time}>{formatTime(order.timestamp)}</span>
              <button
                style={styles.cancelBtn}
                onClick={() => onCancel(order.id)}
                disabled={cancelling[order.id]}
              >
                {cancelling[order.id] ? '...' : '✕'}
              </button>
            </div>
          )
        })}
        {orders.length === 0 && (
          <div style={styles.empty}>No open orders</div>
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
    gridTemplateColumns: '50px 70px 70px 70px 30px',
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
    gridTemplateColumns: '50px 70px 70px 70px 30px',
    padding: '6px 4px',
    fontSize: '13px',
    fontFamily: 'monospace',
    borderRadius: '2px',
    alignItems: 'center',
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
  cancelBtn: {
    background: 'transparent',
    border: '1px solid #444',
    color: '#888',
    cursor: 'pointer',
    borderRadius: '3px',
    padding: '2px 6px',
    fontSize: '12px',
  },
  empty: {
    color: '#666',
    textAlign: 'center',
    padding: '20px',
  },
}

export default OpenOrders
