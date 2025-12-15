import { useState } from 'react'

interface Props {
  onAuth: (token: string, userId: string, accountId: string, username: string) => void
}

function AuthForm({ onAuth }: Props) {
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const endpoint = mode === 'login' ? '/api/auth/login' : '/api/auth/register'
      const resp = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })

      if (!resp.ok) {
        const text = await resp.text()
        throw new Error(text)
      }

      const data = await resp.json()
      onAuth(data.token, data.user_id, data.account_id, data.username)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Authentication failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={styles.container}>
      <div style={styles.card}>
        <h1 style={styles.title}>$FAKE Trading</h1>
        <p style={styles.subtitle}>PvP Paper Trading Game</p>

        <div style={styles.tabs}>
          <button
            style={{
              ...styles.tab,
              ...(mode === 'login' ? styles.activeTab : {}),
            }}
            onClick={() => setMode('login')}
          >
            Login
          </button>
          <button
            style={{
              ...styles.tab,
              ...(mode === 'register' ? styles.activeTab : {}),
            }}
            onClick={() => setMode('register')}
          >
            Register
          </button>
        </div>

        <form onSubmit={handleSubmit} style={styles.form}>
          <div style={styles.field}>
            <label style={styles.label}>Username</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              style={styles.input}
              placeholder="Enter username"
              minLength={3}
              maxLength={32}
              required
            />
          </div>

          <div style={styles.field}>
            <label style={styles.label}>Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              style={styles.input}
              placeholder="Enter password"
              minLength={6}
              required
            />
          </div>

          {error && <div style={styles.error}>{error}</div>}

          <button type="submit" style={styles.button} disabled={loading}>
            {loading ? 'Loading...' : mode === 'login' ? 'Login' : 'Create Account'}
          </button>
        </form>

        <p style={styles.info}>
          {mode === 'register'
            ? 'New accounts start with $1,000,000'
            : 'Trade against other players'}
        </p>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '20px',
  },
  card: {
    background: '#111',
    borderRadius: '12px',
    padding: '32px',
    width: '100%',
    maxWidth: '400px',
  },
  title: {
    fontSize: '28px',
    fontWeight: 'bold',
    textAlign: 'center',
    marginBottom: '8px',
  },
  subtitle: {
    fontSize: '14px',
    color: '#888',
    textAlign: 'center',
    marginBottom: '24px',
  },
  tabs: {
    display: 'flex',
    gap: '8px',
    marginBottom: '24px',
  },
  tab: {
    flex: 1,
    padding: '10px',
    background: '#222',
    border: 'none',
    borderRadius: '6px',
    color: '#888',
    fontSize: '14px',
    cursor: 'pointer',
  },
  activeTab: {
    background: '#333',
    color: '#fff',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '16px',
  },
  field: {
    display: 'flex',
    flexDirection: 'column',
    gap: '6px',
  },
  label: {
    fontSize: '12px',
    color: '#888',
  },
  input: {
    padding: '12px',
    background: '#222',
    border: '1px solid #333',
    borderRadius: '6px',
    color: '#fff',
    fontSize: '16px',
  },
  button: {
    padding: '14px',
    background: '#22c55e',
    border: 'none',
    borderRadius: '6px',
    color: '#fff',
    fontSize: '16px',
    fontWeight: 'bold',
    cursor: 'pointer',
    marginTop: '8px',
  },
  error: {
    padding: '10px',
    background: 'rgba(239, 68, 68, 0.1)',
    border: '1px solid #ef4444',
    borderRadius: '6px',
    color: '#ef4444',
    fontSize: '14px',
  },
  info: {
    fontSize: '12px',
    color: '#666',
    textAlign: 'center',
    marginTop: '16px',
  },
}

export default AuthForm
