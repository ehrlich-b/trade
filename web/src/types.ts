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

export type WSMessage =
  | { type: 'book'; book: BookSnapshot }
  | { type: 'trade'; trade: Trade }
