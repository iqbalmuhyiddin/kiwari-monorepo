package ws

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

// Event represents a WebSocket message to be broadcast
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// outletEvent is an internal struct for routing events to specific outlets
type outletEvent struct {
	OutletID uuid.UUID
	Event    Event
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients by outlet ID
	rooms map[uuid.UUID]map[*Client]bool

	// Inbound messages from clients (register/unregister)
	register   chan *Client
	unregister chan *Client

	// Outbound messages to broadcast
	broadcast chan *outletEvent

	// Mutex for thread-safe room access
	mu sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *outletEvent, 256),
	}
}

// Run starts the hub's main loop
// This should be called as a goroutine: go hub.Run()
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.rooms[client.outletID] == nil {
				h.rooms[client.outletID] = make(map[*Client]bool)
			}
			h.rooms[client.outletID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.rooms[client.outletID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					// Clean up empty rooms
					if len(clients) == 0 {
						delete(h.rooms, client.outletID)
					}
				}
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.mu.Lock()
			clients := h.rooms[event.OutletID]

			// Marshal event to JSON once
			message, err := json.Marshal(event.Event)
			if err != nil {
				h.mu.Unlock()
				continue
			}

			// Send to all clients in this outlet's room
			for client := range clients {
				select {
				case client.send <- message:
				default:
					// Client's send buffer is full, close and unregister
					close(client.send)
					delete(h.rooms[event.OutletID], client)
					if len(h.rooms[event.OutletID]) == 0 {
						delete(h.rooms, event.OutletID)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastToOutlet sends an event to all clients subscribed to a specific outlet
// This is the public API for handlers to broadcast events
func (h *Hub) BroadcastToOutlet(outletID uuid.UUID, event Event) {
	h.broadcast <- &outletEvent{
		OutletID: outletID,
		Event:    event,
	}
}
