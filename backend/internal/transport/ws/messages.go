package ws

// Standard WebSocket broadcast message types.
const (
	MessageTypePriceUpdate    = "PRICE_UPDATE"
	MessageTypeTradeEvent     = "TRADE_EVENT"
	MessageTypeMarketResolved = "MARKET_RESOLVED"
	MessageTypeMarketUpdated  = "MARKET_UPDATED"
	MessageTypeMarketDeleted  = "MARKET_DELETED"
	MessageTypePing           = "PING"
	MessageTypePong           = "PONG"
)

// ReservesPayload represents the outcome reserves snapshot in a price update frame.
type ReservesPayload struct {
	Yes string `json:"yes"`
	No  string `json:"no"`
}

// PriceUpdateMessage encapsulates real-time spot price shifts and virtual inventory state.
type PriceUpdateMessage struct {
	Type            string           `json:"type"`
	MarketID        string           `json:"market_id"`
	YesPrice        string           `json:"yes_price"`
	NoPrice         string           `json:"no_price"`
	Reserves        *ReservesPayload `json:"reserves,omitempty"`
	TotalVolumeUSDC string           `json:"total_volume_usdc,omitempty"`
	Timestamp       string           `json:"timestamp"`
}

// TradeEventMessage encapsulates verified execution details for live tickers and candlestick updates.
type TradeEventMessage struct {
	Type       string `json:"type"`
	TradeID    string `json:"trade_id"`
	MarketID   string `json:"market_id"`
	TradeType  string `json:"trade_type"` // "BUY" | "SELL"
	Outcome    string `json:"outcome"`    // "YES" | "NO"
	Shares     string `json:"shares"`
	Price      string `json:"price"`
	AmountUSDC string `json:"amount_usdc"`
	Timestamp  string `json:"timestamp"`
}

// MarketResolvedMessage encapsulates winning outcome announcements.
type MarketResolvedMessage struct {
	Type           string `json:"type"`
	MarketID       string `json:"market_id"`
	WinningOutcome string `json:"winning_outcome"`
	Timestamp      string `json:"timestamp"`
}

// MarketUpdatedMessage encapsulates administrative market changes.
type MarketUpdatedMessage struct {
	Type        string `json:"type"`
	MarketID    string `json:"market_id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Status      string `json:"status,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// MarketDeletedMessage encapsulates market removal notifications.
type MarketDeletedMessage struct {
	Type      string `json:"type"`
	MarketID  string `json:"market_id"`
	Timestamp string `json:"timestamp"`
}
