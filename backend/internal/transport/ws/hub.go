package ws

import (
	"encoding/json"
	"log"
	"sync"
)

// BroadcastPayload pairs raw message bytes with an optional market ID for targeted fan-out.
type BroadcastPayload struct {
	MarketID string
	Data     []byte
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients: map[client]bool
	clients map[*Client]bool

	// Global client subscriptions (marketID == "" or "all"): map[client]bool
	globalClients map[*Client]bool

	// Market-specific client subscriptions: map[marketID]map[client]bool
	marketClients map[string]map[*Client]bool

	// Inbound messages to be dispatched to clients.
	broadcast chan BroadcastPayload

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Channel to signal graceful shutdown.
	shutdown chan struct{}

	// Protects count queries
	mu sync.RWMutex
}

// NewHub initializes a new WebSocket broker hub instance.
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		globalClients: make(map[*Client]bool),
		marketClients: make(map[string]map[*Client]bool),
		broadcast:     make(chan BroadcastPayload, 1024),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		shutdown:      make(chan struct{}),
	}
}

// Run executes the main event loop of the Hub.
func (h *Hub) Run() {
	for {
		select {
		case <-h.shutdown:
			h.mu.Lock()
			for client := range h.clients {
				client.Close()
				delete(h.clients, client)
			}
			h.globalClients = make(map[*Client]bool)
			h.marketClients = make(map[string]map[*Client]bool)
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.marketID == "" || client.marketID == "all" {
				h.globalClients[client] = true
			} else {
				if _, ok := h.marketClients[client.marketID]; !ok {
					h.marketClients[client.marketID] = make(map[*Client]bool)
				}
				h.marketClients[client.marketID][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			h.removeClient(client)
			h.mu.Unlock()

		case payload := <-h.broadcast:
			h.mu.RLock()
			// O(K) Fan-out: collect target clients without scanning all connected users
			targets := make(map[*Client]struct{})

			if payload.MarketID == "" || payload.MarketID == "all" {
				for c := range h.clients {
					targets[c] = struct{}{}
				}
			} else {
				// Market-specific subscribers
				if mClients, ok := h.marketClients[payload.MarketID]; ok {
					for c := range mClients {
						targets[c] = struct{}{}
					}
				}
				// Plus global subscribers directly from indexed set
				for c := range h.globalClients {
					targets[c] = struct{}{}
				}
			}
			h.mu.RUnlock()

			// Non-blocking fan-out: if client channel buffer is full, immediately disconnect
			// stalled/slow consumer without blocking the central trade execution engine.
			var stalledClients []*Client
			for client := range targets {
				select {
				case client.send <- payload.Data:
				default:
					// Stalled/slow consumer: queue for eviction
					stalledClients = append(stalledClients, client)
				}
			}

			if len(stalledClients) > 0 {
				h.mu.Lock()
				for _, client := range stalledClients {
					log.Printf("[WARN] WS client stalled or buffer full; evicting client for market %s", client.marketID)
					h.removeClient(client)
				}
				h.mu.Unlock()
			}
		}
	}
}

// removeClient removes a client from the hub and cleanly terminates its connection.
// Must be called with h.mu write lock held.
func (h *Hub) removeClient(client *Client) {
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		delete(h.globalClients, client)
		if client.marketID != "" && client.marketID != "all" {
			if mClients, exists := h.marketClients[client.marketID]; exists {
				delete(mClients, client)
				if len(mClients) == 0 {
					delete(h.marketClients, client.marketID)
				}
			}
		}
		client.Close()
	}
}

// Stop initiates a clean shutdown of the Hub and terminates active client loops.
func (h *Hub) Stop() {
	select {
	case <-h.shutdown:
		// already stopped
	default:
		close(h.shutdown)
	}
}

// ClientCount returns the total number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// MarketClientCount returns the number of connected clients subscribed to a specific market.
func (h *Hub) MarketClientCount(marketID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if mClients, ok := h.marketClients[marketID]; ok {
		return len(mClients)
	}
	return 0
}

// Broadcast dispatches raw bytes to subscribers.
func (h *Hub) Broadcast(marketID string, data []byte) {
	select {
	case h.broadcast <- BroadcastPayload{MarketID: marketID, Data: data}:
	default:
		log.Println("[WARN] WS hub broadcast queue full, dropping frame")
	}
}

// BroadcastPriceUpdate formats and broadcasts a PRICE_UPDATE telemetry frame.
func (h *Hub) BroadcastPriceUpdate(msg PriceUpdateMessage) {
	msg.Type = MessageTypePriceUpdate
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal PriceUpdateMessage: %v", err)
		return
	}
	h.Broadcast(msg.MarketID, data)
}

// BroadcastTradeEvent formats and broadcasts a TRADE_EVENT telemetry frame.
func (h *Hub) BroadcastTradeEvent(msg TradeEventMessage) {
	msg.Type = MessageTypeTradeEvent
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal TradeEventMessage: %v", err)
		return
	}
	h.Broadcast(msg.MarketID, data)
}

// BroadcastMarketResolved formats and broadcasts a MARKET_RESOLVED telemetry frame.
func (h *Hub) BroadcastMarketResolved(msg MarketResolvedMessage) {
	msg.Type = MessageTypeMarketResolved
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal MarketResolvedMessage: %v", err)
		return
	}
	h.Broadcast(msg.MarketID, data)
}
