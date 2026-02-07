package ws

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kiwari-pos/api/internal/auth"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (we validate via JWT)
	},
}

// Client represents a single WebSocket connection
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	outletID uuid.UUID
	send     chan []byte
}

// ReadPump pumps messages from the WebSocket connection to the hub
// The application runs ReadPump in a per-connection goroutine
// For a POS system, clients don't send messages - we just detect disconnects
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Read loop - we just wait for disconnect or errors
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
// The application runs WritePump in a per-connection goroutine
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWS handles WebSocket requests from clients
// Endpoint: WS /ws/outlets/:oid/orders?token=JWT
func ServeWS(hub *Hub, jwtSecret string, w http.ResponseWriter, r *http.Request) {
	// 1. Extract token from query param
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	// 2. Validate JWT
	claims, err := auth.ValidateToken(jwtSecret, tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// 3. Extract outlet ID from URL
	oidStr := chi.URLParam(r, "oid")
	outletID, err := uuid.Parse(oidStr)
	if err != nil {
		http.Error(w, "invalid outlet id", http.StatusBadRequest)
		return
	}

	// 4. Verify outlet access (OWNER can access any outlet, others only their own)
	if claims.Role != "OWNER" && claims.OutletID != outletID {
		http.Error(w, "outlet access denied", http.StatusForbidden)
		return
	}

	// 5. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	// 6. Create client and register with hub
	client := &Client{
		hub:      hub,
		conn:     conn,
		outletID: outletID,
		send:     make(chan []byte, 256),
	}
	client.hub.register <- client

	// 7. Start pumps in separate goroutines
	go client.WritePump()
	go client.ReadPump()
}
