export interface LevelSnapshot {
  price: number
  quantity: number
}

export interface BookSnapshot {
  symbol: string
  bids: LevelSnapshot[]
  asks: LevelSnapshot[]
}

export interface Trade {
  id: string
  symbol: string
  price: number
  quantity: number
  buy_order_id: string
  sell_order_id: string
  buyer_id: string
  seller_id: string
  timestamp: string
}

export interface Position {
  quantity: number
  avgPrice: number
  pnl: number
}

export interface Order {
  id: string
  user_id: string
  symbol: string
  side: number // 0 = Buy, 1 = Sell
  type: number // 0 = Limit, 1 = Market
  price: number // in cents
  quantity: number
  filled: number
  timestamp: string
}

// Match-related types
export type MatchState = 'LOBBY' | 'PRE_MATCH' | 'TRADING' | 'SETTLEMENT' | 'COMPLETE'

export interface MatchParticipant {
  user_id: string
  starting_shares: number
  starting_cash: number
}

export interface MatchResult {
  user_id: string
  rank: number
  pnl: number
  final_value: number
  start_value: number
  start_shares: number
  final_shares: number
}

// OHLCV bar data for candlestick charts
export interface BarData {
  time: string    // Market time string (e.g., "9:30 AM")
  open: number    // Price in cents
  high: number
  low: number
  close: number
  volume: number
}

export interface MatchStateMessage {
  type: 'match_state'
  match_id: string
  state: MatchState
  symbol: string
  duration: number
  nav: number
  participants: MatchParticipant[]
  bars?: BarData[]       // Price history
  current_bar?: number   // Current bar index
}

export interface CountdownMessage {
  type: 'countdown'
  match_id: string
  seconds: number
}

export interface PriceTickMessage {
  type: 'price_tick'
  match_id: string
  nav: number
  market_time: string
  remaining_sec: number
  progress: number
}

export interface MatchResultsMessage {
  type: 'match_results'
  match_id: string
  final_nav: number
  results: MatchResult[]
}

export type WSMessage =
  | { type: 'book'; book: BookSnapshot }
  | { type: 'trade'; trade: Trade }
  | MatchStateMessage
  | CountdownMessage
  | PriceTickMessage
  | MatchResultsMessage
