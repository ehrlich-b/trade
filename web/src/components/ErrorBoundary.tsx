import { Component, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo)
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={styles.container}>
          <div style={styles.card}>
            <h1 style={styles.title}>Something went wrong</h1>
            <p style={styles.message}>
              The application encountered an unexpected error.
            </p>
            {this.state.error && (
              <pre style={styles.error}>{this.state.error.message}</pre>
            )}
            <button onClick={this.handleRetry} style={styles.button}>
              Try Again
            </button>
            <button onClick={() => window.location.reload()} style={styles.secondaryButton}>
              Reload Page
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '20px',
    backgroundColor: '#0a0a0a',
  },
  card: {
    background: '#111',
    borderRadius: '12px',
    padding: '32px',
    maxWidth: '500px',
    textAlign: 'center',
  },
  title: {
    fontSize: '24px',
    fontWeight: 'bold',
    color: '#ef4444',
    marginBottom: '16px',
  },
  message: {
    fontSize: '14px',
    color: '#888',
    marginBottom: '16px',
  },
  error: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '6px',
    padding: '12px',
    fontSize: '12px',
    color: '#f87171',
    textAlign: 'left',
    overflow: 'auto',
    maxHeight: '150px',
    marginBottom: '16px',
  },
  button: {
    padding: '12px 24px',
    background: '#22c55e',
    border: 'none',
    borderRadius: '6px',
    color: '#fff',
    fontSize: '14px',
    fontWeight: 'bold',
    cursor: 'pointer',
    marginRight: '8px',
  },
  secondaryButton: {
    padding: '12px 24px',
    background: '#333',
    border: 'none',
    borderRadius: '6px',
    color: '#888',
    fontSize: '14px',
    cursor: 'pointer',
  },
}

export default ErrorBoundary
