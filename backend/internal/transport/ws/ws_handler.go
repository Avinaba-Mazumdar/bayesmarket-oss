package ws

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const defaultMaxWSConnectionsPerIP = 20

// WSHandler manages HTTP to WebSocket upgrades.
type WSHandler struct {
	hub            *Hub
	upgrader       websocket.Upgrader
	allowedOrigins []string
	isDev          bool
	ipConnMu       sync.Mutex
	ipConns        map[string]int
	maxConnsPerIP  int
}

// NewWSHandler creates a new WebSocket handler instance.
func NewWSHandler(hub *Hub, allowedOrigin string, isDev ...bool) *WSHandler {
	var origins []string
	for _, o := range strings.Split(allowedOrigin, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}

	devMode := false
	if len(isDev) > 0 && isDev[0] {
		devMode = true
	}

	h := &WSHandler{
		hub:            hub,
		allowedOrigins: origins,
		isDev:          devMode,
		ipConns:        make(map[string]int),
		maxConnsPerIP:  defaultMaxWSConnectionsPerIP,
	}

	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			for _, allowed := range h.allowedOrigins {
				if allowed == "*" || strings.EqualFold(origin, allowed) {
					return true
				}
			}
			if h.isDev {
				lower := strings.ToLower(origin)
				if strings.HasPrefix(lower, "http://localhost:") || lower == "http://localhost" ||
					strings.HasPrefix(lower, "http://127.0.0.1:") || lower == "http://127.0.0.1" {
					return true
				}
			}
			return false
		},
	}

	return h
}

// HandleMarketWS upgrades connections requesting market-specific telemetry.
//
// GET /ws/markets/:id
func (h *WSHandler) HandleMarketWS(c *gin.Context) {
	marketID := strings.TrimSpace(c.Param("id"))
	h.serveWS(c, marketID)
}

// HandleGlobalWS upgrades connections requesting the global stream across all markets.
//
// GET /ws/markets or GET /ws
func (h *WSHandler) HandleGlobalWS(c *gin.Context) {
	h.serveWS(c, "all")
}

func (h *WSHandler) serveWS(c *gin.Context, marketID string) {
	clientIP := c.ClientIP()

	h.ipConnMu.Lock()
	if h.ipConns[clientIP] >= h.maxConnsPerIP {
		h.ipConnMu.Unlock()
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error":   "too_many_ws_connections",
			"message": "Maximum concurrent WebSocket connections reached for this client IP",
		})
		return
	}
	h.ipConns[clientIP]++
	h.ipConnMu.Unlock()

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.ipConnMu.Lock()
		h.ipConns[clientIP]--
		if h.ipConns[clientIP] <= 0 {
			delete(h.ipConns, clientIP)
		}
		h.ipConnMu.Unlock()
		log.Printf("[WARN] Failed to upgrade WebSocket connection: %v", err)
		return
	}

	client := NewClient(h.hub, conn, marketID)
	client.SetOnClose(func() {
		h.ipConnMu.Lock()
		if count, ok := h.ipConns[clientIP]; ok {
			if count <= 1 {
				delete(h.ipConns, clientIP)
			} else {
				h.ipConns[clientIP] = count - 1
			}
		}
		h.ipConnMu.Unlock()
	})

	h.hub.register <- client

	// Start pump goroutines
	go client.writePump()
	go client.readPump()
}
