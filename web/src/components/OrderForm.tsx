import { useState } from 'react'

interface Props {
  onSubmit: (side: 'buy' | 'sell', type: 'limit' | 'market', price: number, quantity: number) => void
  midPrice: number
}

function OrderForm({ onSubmit, midPrice }: Props) {
  const [side, setSide] = useState<'buy' | 'sell'>('buy')
  const [type, setType] = useState<'limit' | 'market'>('limit')
  const [price, setPrice] = useState(midPrice / 100)
  const [quantity, setQuantity] = useState(10)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(side, type, Math.round(price * 100), quantity)
  }

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Place Order</h2>

      <form onSubmit={handleSubmit} style={styles.form}>
        <div style={styles.sideButtons}>
          <button
            type="button"
            onClick={() => setSide('buy')}
            style={{
              ...styles.sideButton,
              ...(side === 'buy' ? styles.buyActive : styles.inactive),
            }}
          >
            BUY
          </button>
          <button
            type="button"
            onClick={() => setSide('sell')}
            style={{
              ...styles.sideButton,
              ...(side === 'sell' ? styles.sellActive : styles.inactive),
            }}
          >
            SELL
          </button>
        </div>

        <div style={styles.typeButtons}>
          <button
            type="button"
            onClick={() => setType('limit')}
            style={{
              ...styles.typeButton,
              ...(type === 'limit' ? styles.typeActive : styles.inactive),
            }}
          >
            Limit
          </button>
          <button
            type="button"
            onClick={() => setType('market')}
            style={{
              ...styles.typeButton,
              ...(type === 'market' ? styles.typeActive : styles.inactive),
            }}
          >
            Market
          </button>
        </div>

        {type === 'limit' && (
          <div style={styles.field}>
            <label style={styles.label}>Price ($)</label>
            <input
              type="number"
              step="0.01"
              value={price}
              onChange={(e) => setPrice(parseFloat(e.target.value) || 0)}
              style={styles.input}
            />
          </div>
        )}

        <div style={styles.field}>
          <label style={styles.label}>Quantity</label>
          <input
            type="number"
            value={quantity}
            onChange={(e) => setQuantity(parseInt(e.target.value) || 0)}
            style={styles.input}
          />
        </div>

        <button
          type="submit"
          style={{
            ...styles.submitButton,
            backgroundColor: side === 'buy' ? '#22c55e' : '#ef4444',
          }}
        >
          {side === 'buy' ? 'BUY' : 'SELL'} {quantity} @ {type === 'market' ? 'MKT' : `$${price.toFixed(2)}`}
        </button>
      </form>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    background: '#111',
    borderRadius: '8px',
    padding: '16px',
  },
  title: {
    fontSize: '14px',
    fontWeight: 'bold',
    marginBottom: '16px',
    color: '#888',
    textTransform: 'uppercase',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
  },
  sideButtons: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '8px',
  },
  sideButton: {
    padding: '12px',
    border: 'none',
    borderRadius: '4px',
    fontSize: '14px',
    fontWeight: 'bold',
    cursor: 'pointer',
  },
  buyActive: {
    backgroundColor: '#22c55e',
    color: '#fff',
  },
  sellActive: {
    backgroundColor: '#ef4444',
    color: '#fff',
  },
  inactive: {
    backgroundColor: '#222',
    color: '#666',
  },
  typeButtons: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '8px',
  },
  typeButton: {
    padding: '8px',
    border: 'none',
    borderRadius: '4px',
    fontSize: '12px',
    cursor: 'pointer',
  },
  typeActive: {
    backgroundColor: '#333',
    color: '#fff',
  },
  field: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
  },
  label: {
    fontSize: '12px',
    color: '#888',
  },
  input: {
    padding: '10px',
    backgroundColor: '#222',
    border: '1px solid #333',
    borderRadius: '4px',
    color: '#fff',
    fontSize: '16px',
  },
  submitButton: {
    padding: '14px',
    border: 'none',
    borderRadius: '4px',
    fontSize: '14px',
    fontWeight: 'bold',
    color: '#fff',
    cursor: 'pointer',
    marginTop: '8px',
  },
}

export default OrderForm
