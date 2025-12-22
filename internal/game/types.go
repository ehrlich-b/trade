package game

import "trade/internal/match"

// Re-export match types for convenience
type Match = match.Match
type State = match.State
type PriceFeed = match.PriceFeed

const (
	StateLobby      = match.StateLobby
	StatePreMatch   = match.StatePreMatch
	StateTrading    = match.StateTrading
	StateSettlement = match.StateSettlement
	StateComplete   = match.StateComplete
)
