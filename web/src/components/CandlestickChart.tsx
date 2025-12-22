import { useMemo, useState, useRef, useEffect } from 'react'
import { BarData } from '../types'

interface CandlestickChartProps {
  bars: BarData[]
  currentBar: number
}

// Constants outside component to avoid recreation
const PADDING = { top: 10, right: 60, bottom: 20, left: 10 }
const VOLUME_HEIGHT = 40
const MIN_HEIGHT = 280

// Time range options (in minutes of in-game time)
type TimeRange = '15m' | '30m' | '1h' | 'all'
// Candle granularity (in minutes)
type Granularity = '1m' | '5m'

export default function CandlestickChart({
  bars,
  currentBar: _currentBar,
}: CandlestickChartProps) {
  void _currentBar

  const containerRef = useRef<HTMLDivElement>(null)
  const [dimensions, setDimensions] = useState({ width: 600, height: MIN_HEIGHT })
  const [timeRange, setTimeRange] = useState<TimeRange>('1h')
  const [granularity, setGranularity] = useState<Granularity>('1m')

  // Measure container on mount and resize
  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const updateDimensions = () => {
      const rect = container.getBoundingClientRect()
      setDimensions({
        width: Math.max(300, rect.width),
        height: Math.max(MIN_HEIGHT, rect.height - 40), // Account for controls
      })
    }

    updateDimensions()

    const resizeObserver = new ResizeObserver(updateDimensions)
    resizeObserver.observe(container)

    return () => resizeObserver.disconnect()
  }, [])

  const { width, height } = dimensions
  const chartHeight = height - 60

  // Filter and aggregate bars based on time range and granularity
  const filteredBars = useMemo(() => {
    if (!bars || bars.length === 0) return []

    // Filter by time range (each bar is 1 minute of in-game time)
    let maxBars = bars.length
    switch (timeRange) {
      case '15m':
        maxBars = 15
        break
      case '30m':
        maxBars = 30
        break
      case '1h':
        maxBars = 60
        break
      case 'all':
        maxBars = bars.length
        break
    }

    const slicedBars = bars.slice(-Math.min(maxBars, bars.length))

    // Aggregate by granularity (5m = combine every 5 bars into 1)
    if (granularity === '1m') {
      return slicedBars
    }

    // 5-minute candles
    const aggregated: BarData[] = []
    for (let i = 0; i < slicedBars.length; i += 5) {
      const chunk = slicedBars.slice(i, Math.min(i + 5, slicedBars.length))
      if (chunk.length === 0) continue
      aggregated.push({
        time: chunk[0].time,
        open: chunk[0].open,
        high: Math.max(...chunk.map(c => c.high)),
        low: Math.min(...chunk.map(c => c.low)),
        close: chunk[chunk.length - 1].close,
        volume: chunk.reduce((sum, c) => sum + c.volume, 0),
      })
    }
    return aggregated
  }, [bars, timeRange, granularity])

  // Compute all chart data in a single useMemo
  const chartData = useMemo(() => {
    if (!filteredBars || filteredBars.length === 0) {
      return null
    }

    // Calculate price range with some padding
    const prices = filteredBars.flatMap((b) => [b.open, b.high, b.low, b.close])
    const minPrice = Math.min(...prices)
    const maxPrice = Math.max(...prices)
    const priceBuffer = (maxPrice - minPrice) * 0.1 || 100
    const priceMin = minPrice - priceBuffer
    const priceMax = maxPrice + priceBuffer
    const priceRangeVal = priceMax - priceMin

    // Volume max
    const volumeMax = Math.max(...filteredBars.map((b) => b.volume), 1)

    // Calculate candle width based on available space
    const chartWidth = width - PADDING.left - PADDING.right
    const candleWidth = Math.max(2, Math.min(12, chartWidth / Math.max(filteredBars.length, 1) - 2))
    const step = chartWidth / Math.max(filteredBars.length, 1)

    // Price labels
    const priceLabels: { y: number; label: string }[] = []
    const steps = 5
    for (let i = 0; i <= steps; i++) {
      const price = priceMax - (priceRangeVal * i) / steps
      priceLabels.push({
        y: PADDING.top + (chartHeight * i) / steps,
        label: `$${(price / 100).toFixed(2)}`,
      })
    }

    // Pre-compute candle positions and dimensions
    const candles = filteredBars.map((bar, i) => {
      const x = PADDING.left + step * i + step / 2
      const isUp = bar.close >= bar.open

      // Price to Y coordinate
      const toY = (price: number) => {
        const normalized = (priceMax - price) / priceRangeVal
        return PADDING.top + normalized * chartHeight
      }

      const wickTop = toY(bar.high)
      const wickBottom = toY(bar.low)
      const bodyTop = toY(Math.max(bar.open, bar.close))
      const bodyBottom = toY(Math.min(bar.open, bar.close))
      const bodyHeight = Math.max(1, bodyBottom - bodyTop)

      // Volume bar
      const volHeight = (bar.volume / volumeMax) * VOLUME_HEIGHT

      return {
        x,
        isUp,
        wickTop,
        wickBottom,
        bodyTop,
        bodyHeight,
        volHeight,
        candleWidth,
      }
    })

    // Current price
    const currentPrice = filteredBars[filteredBars.length - 1].close
    const currentPriceY = PADDING.top + ((priceMax - currentPrice) / priceRangeVal) * chartHeight

    return {
      priceLabels,
      candles,
      currentPrice,
      currentPriceY,
      firstTime: filteredBars[0].time,
      lastTime: filteredBars[filteredBars.length - 1].time,
    }
  }, [filteredBars, width, height, chartHeight])

  if (!chartData) {
    return (
      <div ref={containerRef} style={styles.container}>
        <div style={styles.header}>
          <span style={styles.title}>PRICE CHART</span>
          <div style={styles.controls}>
            <TimeRangeSelector value={timeRange} onChange={setTimeRange} />
            <GranularitySelector value={granularity} onChange={setGranularity} />
          </div>
        </div>
        <div style={styles.noData}>Waiting for price data...</div>
      </div>
    )
  }

  const { priceLabels, candles, currentPrice, currentPriceY, firstTime, lastTime } = chartData

  return (
    <div ref={containerRef} style={styles.container}>
      <div style={styles.header}>
        <span style={styles.title}>PRICE CHART</span>
        <div style={styles.controls}>
          <TimeRangeSelector value={timeRange} onChange={setTimeRange} />
          <GranularitySelector value={granularity} onChange={setGranularity} />
          <span style={styles.currentPrice}>
            ${(currentPrice / 100).toFixed(2)}
          </span>
        </div>
      </div>
      <svg width={width} height={height} style={styles.svg}>
        {/* Grid lines */}
        {priceLabels.map((label, i) => (
          <line
            key={i}
            x1={PADDING.left}
            y1={label.y}
            x2={width - PADDING.right}
            y2={label.y}
            stroke="#222"
            strokeDasharray="2,2"
          />
        ))}

        {/* Price labels */}
        {priceLabels.map((label, i) => (
          <text
            key={`label-${i}`}
            x={width - PADDING.right + 5}
            y={label.y + 4}
            fill="#666"
            fontSize="10"
            fontFamily="monospace"
          >
            {label.label}
          </text>
        ))}

        {/* Volume bars */}
        {candles.map((c, i) => (
          <rect
            key={`vol-${i}`}
            x={c.x - c.candleWidth / 2}
            y={chartHeight + PADDING.top + VOLUME_HEIGHT - c.volHeight}
            width={c.candleWidth}
            height={c.volHeight}
            fill={c.isUp ? 'rgba(34, 197, 94, 0.3)' : 'rgba(239, 68, 68, 0.3)'}
          />
        ))}

        {/* Candlesticks */}
        {candles.map((c, i) => {
          const color = c.isUp ? '#22c55e' : '#ef4444'
          return (
            <g key={`candle-${i}`}>
              <line
                x1={c.x}
                y1={c.wickTop}
                x2={c.x}
                y2={c.wickBottom}
                stroke={color}
                strokeWidth={1}
              />
              <rect
                x={c.x - c.candleWidth / 2}
                y={c.bodyTop}
                width={c.candleWidth}
                height={c.bodyHeight}
                fill={color}
                stroke={color}
                strokeWidth={1}
              />
            </g>
          )
        })}

        {/* Current price line */}
        <line
          x1={PADDING.left}
          y1={currentPriceY}
          x2={width - PADDING.right}
          y2={currentPriceY}
          stroke="#3b82f6"
          strokeWidth={1}
          strokeDasharray="4,2"
        />
        <rect
          x={width - PADDING.right}
          y={currentPriceY - 8}
          width={55}
          height={16}
          fill="#3b82f6"
          rx={2}
        />
        <text
          x={width - PADDING.right + 5}
          y={currentPriceY + 4}
          fill="#fff"
          fontSize="10"
          fontFamily="monospace"
          fontWeight="bold"
        >
          ${(currentPrice / 100).toFixed(2)}
        </text>

        {/* Time labels at bottom */}
        <text
          x={PADDING.left}
          y={height - 5}
          fill="#666"
          fontSize="9"
          fontFamily="monospace"
        >
          {firstTime}
        </text>
        <text
          x={width - PADDING.right}
          y={height - 5}
          fill="#666"
          fontSize="9"
          fontFamily="monospace"
          textAnchor="end"
        >
          {lastTime}
        </text>
      </svg>
    </div>
  )
}

// Time Range Selector component
function TimeRangeSelector({
  value,
  onChange
}: {
  value: TimeRange
  onChange: (v: TimeRange) => void
}) {
  const options: TimeRange[] = ['15m', '30m', '1h', 'all']
  return (
    <div style={selectorStyles.group}>
      {options.map(opt => (
        <button
          key={opt}
          onClick={() => onChange(opt)}
          style={{
            ...selectorStyles.button,
            ...(value === opt ? selectorStyles.buttonActive : {}),
          }}
        >
          {opt.toUpperCase()}
        </button>
      ))}
    </div>
  )
}

// Granularity Selector component
function GranularitySelector({
  value,
  onChange
}: {
  value: Granularity
  onChange: (v: Granularity) => void
}) {
  const options: Granularity[] = ['1m', '5m']
  return (
    <div style={selectorStyles.group}>
      {options.map(opt => (
        <button
          key={opt}
          onClick={() => onChange(opt)}
          style={{
            ...selectorStyles.button,
            ...(value === opt ? selectorStyles.buttonActive : {}),
          }}
        >
          {opt.toUpperCase()}
        </button>
      ))}
    </div>
  )
}

const selectorStyles: Record<string, React.CSSProperties> = {
  group: {
    display: 'flex',
    gap: '2px',
    background: '#1a1a1a',
    borderRadius: '4px',
    padding: '2px',
  },
  button: {
    padding: '4px 8px',
    background: 'transparent',
    border: 'none',
    borderRadius: '3px',
    color: '#666',
    fontSize: '10px',
    fontWeight: 'bold',
    cursor: 'pointer',
    transition: 'all 0.15s ease',
  },
  buttonActive: {
    background: '#333',
    color: '#fff',
  },
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    background: '#0a0a0a',
    borderRadius: '6px',
    overflow: 'hidden',
    width: '100%',
    height: '100%',
    minHeight: '320px',
    display: 'flex',
    flexDirection: 'column',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '8px 12px',
    borderBottom: '1px solid #222',
    flexShrink: 0,
  },
  title: {
    fontSize: '11px',
    fontWeight: 'bold',
    color: '#666',
    letterSpacing: '0.5px',
  },
  controls: {
    display: 'flex',
    gap: '12px',
    alignItems: 'center',
  },
  currentPrice: {
    fontSize: '14px',
    fontWeight: 'bold',
    fontFamily: 'monospace',
    color: '#fff',
  },
  svg: {
    display: 'block',
    flex: 1,
  },
  noData: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    color: '#666',
    fontSize: '13px',
  },
}
