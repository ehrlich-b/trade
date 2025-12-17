import { useState, useEffect, useRef } from 'react'

interface Props {
  onSubmit: (side: 'buy' | 'sell', type: 'limit' | 'market', price: number, quantity: number) => void
  midPrice: number
  submitting?: boolean
}

function OrderForm({ onSubmit, midPrice, submitting = false }: Props) {
  const [side, setSide] = useState<'buy' | 'sell'>('buy')
  const [type, setType] = useState<'limit' | 'market'>('limit')
  const [price, setPrice] = useState(Math.round(midPrice) / 100)
  const [quantity, setQuantity] = useState(10)
  const [error, setError] = useState<string | null>(null)
  const userEditedPrice = useRef(false)

  // Sync price with midPrice when it changes (unless user has manually edited)
  // Round to 2 decimal places to avoid precision issues
  useEffect(() => {
    if (!userEditedPrice.current) {
      setPrice(Math.round(midPrice) / 100)
    }
  }, [midPrice])

  const validate = (): boolean => {
    if (quantity <= 0 || !Number.isInteger(quantity)) {
      setError('Quantity must be a positive integer')
      return false
    }
    if (type === 'limit' && price <= 0) {
      setError('Price must be positive')
      return false
    }
    setError(null)
    return true
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!validate()) return
    onSubmit(side, type, Math.round(price * 100), quantity)
    userEditedPrice.current = false
  }

  const handlePriceChange = (value: number) => {
    userEditedPrice.current = true
    // Round to 2 decimal places to avoid precision issues
    const rounded = Math.round(value * 100) / 100
    setPrice(rounded)
    setError(null)
  }

  const handleQuantityChange = (value: number) => {
    setQuantity(value)
    setError(null)
  }

  // Quick quantity buttons
  const quickQty = [10, 25, 50, 100]

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>PLACE ORDER</h2>

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
              min="0.01"
              value={price}
              onChange={(e) => handlePriceChange(parseFloat(e.target.value) || 0)}
              style={styles.input}
            />
          </div>
        )}

        <div style={styles.field}>
          <label style={styles.label}>Quantity</label>
          <input
            type="number"
            min="1"
            step="1"
            value={quantity}
            onChange={(e) => handleQuantityChange(parseInt(e.target.value) || 0)}
            style={styles.input}
          />
          <div style={styles.quickButtons}>
            {quickQty.map((q) => (
              <button
                key={q}
                type="button"
                onClick={() => handleQuantityChange(q)}
                style={{
                  ...styles.quickButton,
                  ...(quantity === q ? styles.quickButtonActive : {}),
                }}
              >
                {q}
              </button>
            ))}
          </div>
        </div>

        {error && <div style={styles.error}>{error}</div>}

        <button
          type="submit"
          disabled={submitting}
          style={{
            ...styles.submitButton,
            backgroundColor: side === 'buy' ? '#22c55e' : '#ef4444',
            opacity: submitting ? 0.6 : 1,
            cursor: submitting ? 'not-allowed' : 'pointer',
          }}
        >
          {submitting
            ? 'Submitting...'
            : `${side === 'buy' ? 'BUY' : 'SELL'} ${quantity} @ ${type === 'market' ? 'MKT' : `$${price.toFixed(2)}`}`}
        </button>
      </form>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    // Removed background - parent panel has it
  },
  title: {
    fontSize: '12px',
    fontWeight: 'bold',
    marginBottom: '12px',
    color: '#666',
    letterSpacing: '0.5px',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '10px',
  },
  sideButtons: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '6px',
  },
  sideButton: {
    padding: '10px',
    border: 'none',
    borderRadius: '4px',
    fontSize: '13px',
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
    backgroundColor: '#1a1a1a',
    color: '#666',
  },
  typeButtons: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '6px',
  },
  typeButton: {
    padding: '6px',
    border: 'none',
    borderRadius: '4px',
    fontSize: '11px',
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
    fontSize: '11px',
    color: '#666',
  },
  input: {
    padding: '8px',
    backgroundColor: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '4px',
    color: '#fff',
    fontSize: '14px',
    fontFamily: 'monospace',
  },
  quickButtons: {
    display: 'flex',
    gap: '4px',
    marginTop: '4px',
  },
  quickButton: {
    flex: 1,
    padding: '4px',
    backgroundColor: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '3px',
    color: '#666',
    fontSize: '11px',
    cursor: 'pointer',
  },
  quickButtonActive: {
    backgroundColor: '#333',
    color: '#fff',
    borderColor: '#444',
  },
  error: {
    color: '#ef4444',
    fontSize: '12px',
    padding: '6px 8px',
    backgroundColor: 'rgba(239, 68, 68, 0.1)',
    borderRadius: '4px',
  },
  submitButton: {
    padding: '12px',
    border: 'none',
    borderRadius: '4px',
    fontSize: '13px',
    fontWeight: 'bold',
    color: '#fff',
    cursor: 'pointer',
    marginTop: '4px',
  },
}

export default OrderForm
