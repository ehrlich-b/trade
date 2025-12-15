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

export type WSMessage =
  | { type: 'book'; book: BookSnapshot }
  | { type: 'trade'; trade: Trade }
