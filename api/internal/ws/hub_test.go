package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mockClient creates a client for testing without a real WebSocket connection
func mockClient(hub *Hub, outletID uuid.UUID) *Client {
	return &Client{
		hub:      hub,
		outletID: outletID,
		send:     make(chan []byte, 256),
	}
}

func TestHubRegistration(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	outletID := uuid.New()
	client := mockClient(hub, outletID)

	// Register client
	hub.register <- client

	// Give hub time to process
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	if hub.rooms[outletID] == nil {
		t.Fatal("outlet room not created")
	}
	if !hub.rooms[outletID][client] {
		t.Fatal("client not registered in outlet room")
	}
}

func TestHubUnregistration(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	outletID := uuid.New()
	client := mockClient(hub, outletID)

	// Register client
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Unregister client
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	// Room should be cleaned up when empty
	if hub.rooms[outletID] != nil {
		t.Fatal("outlet room not cleaned up after last client unregistered")
	}
}

func TestBroadcastToSingleOutlet(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	outlet1 := uuid.New()
	outlet2 := uuid.New()

	client1 := mockClient(hub, outlet1)
	client2 := mockClient(hub, outlet2)

	// Register both clients
	hub.register <- client1
	hub.register <- client2
	time.Sleep(10 * time.Millisecond)

	// Broadcast to outlet1 only
	testPayload := json.RawMessage(`{"order_id":"test-123"}`)
	event := Event{
		Type:    "order.created",
		Payload: testPayload,
	}
	hub.BroadcastToOutlet(outlet1, event)

	// Check client1 receives the message
	select {
	case msg := <-client1.send:
		var received Event
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if received.Type != "order.created" {
			t.Errorf("expected type 'order.created', got '%s'", received.Type)
		}
		if string(received.Payload) != string(testPayload) {
			t.Errorf("expected payload '%s', got '%s'", testPayload, received.Payload)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client1 did not receive message")
	}

	// Check client2 does NOT receive the message
	select {
	case <-client2.send:
		t.Fatal("client2 should not have received message for different outlet")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message received
	}
}

func TestBroadcastToMultipleClientsInSameOutlet(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	outletID := uuid.New()
	client1 := mockClient(hub, outletID)
	client2 := mockClient(hub, outletID)
	client3 := mockClient(hub, outletID)

	// Register all clients to same outlet
	hub.register <- client1
	hub.register <- client2
	hub.register <- client3
	time.Sleep(10 * time.Millisecond)

	// Broadcast event
	testPayload := json.RawMessage(`{"status":"READY"}`)
	event := Event{
		Type:    "item.updated",
		Payload: testPayload,
	}
	hub.BroadcastToOutlet(outletID, event)

	// All three clients should receive the message
	clients := []*Client{client1, client2, client3}
	for i, client := range clients {
		select {
		case msg := <-client.send:
			var received Event
			if err := json.Unmarshal(msg, &received); err != nil {
				t.Fatalf("client%d: failed to unmarshal: %v", i+1, err)
			}
			if received.Type != "item.updated" {
				t.Errorf("client%d: expected type 'item.updated', got '%s'", i+1, received.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("client%d did not receive message", i+1)
		}
	}
}

func TestEventSerialization(t *testing.T) {
	testCases := []struct {
		name    string
		event   Event
		wantErr bool
	}{
		{
			name: "order created event",
			event: Event{
				Type:    "order.created",
				Payload: json.RawMessage(`{"id":"abc","total":25000}`),
			},
			wantErr: false,
		},
		{
			name: "order updated event",
			event: Event{
				Type:    "order.updated",
				Payload: json.RawMessage(`{"id":"def","status":"COMPLETED"}`),
			},
			wantErr: false,
		},
		{
			name: "item updated event",
			event: Event{
				Type:    "item.updated",
				Payload: json.RawMessage(`{"item_id":"ghi","kitchen_status":"READY"}`),
			},
			wantErr: false,
		},
		{
			name: "order paid event",
			event: Event{
				Type:    "order.paid",
				Payload: json.RawMessage(`{"order_id":"jkl","amount":50000}`),
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.event)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Marshal error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}

			// Verify we can unmarshal back
			var decoded Event
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if decoded.Type != tc.event.Type {
				t.Errorf("Type mismatch: got %s, want %s", decoded.Type, tc.event.Type)
			}
			if string(decoded.Payload) != string(tc.event.Payload) {
				t.Errorf("Payload mismatch: got %s, want %s", decoded.Payload, tc.event.Payload)
			}
		})
	}
}

func TestHubMultipleOutletsIsolation(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	outlet1 := uuid.New()
	outlet2 := uuid.New()
	outlet3 := uuid.New()

	// Create 2 clients per outlet
	clients := map[uuid.UUID][]*Client{
		outlet1: {mockClient(hub, outlet1), mockClient(hub, outlet1)},
		outlet2: {mockClient(hub, outlet2), mockClient(hub, outlet2)},
		outlet3: {mockClient(hub, outlet3), mockClient(hub, outlet3)},
	}

	// Register all clients
	for _, clientList := range clients {
		for _, client := range clientList {
			hub.register <- client
		}
	}
	time.Sleep(10 * time.Millisecond)

	// Broadcast to outlet2 only
	event := Event{
		Type:    "order.paid",
		Payload: json.RawMessage(`{"outlet_id":"` + outlet2.String() + `"}`),
	}
	hub.BroadcastToOutlet(outlet2, event)

	// Only outlet2 clients should receive
	for outletID, clientList := range clients {
		for i, client := range clientList {
			select {
			case msg := <-client.send:
				if outletID != outlet2 {
					t.Fatalf("outlet %s client %d should not receive message", outletID, i)
				}
				var received Event
				if err := json.Unmarshal(msg, &received); err != nil {
					t.Fatalf("unmarshal error: %v", err)
				}
				if received.Type != "order.paid" {
					t.Errorf("wrong event type: %s", received.Type)
				}
			case <-time.After(50 * time.Millisecond):
				if outletID == outlet2 {
					t.Fatalf("outlet2 client %d should have received message", i)
				}
				// Expected for other outlets
			}
		}
	}
}

func TestHubCleanupEmptyRoom(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	outletID := uuid.New()
	client1 := mockClient(hub, outletID)
	client2 := mockClient(hub, outletID)

	// Register both clients
	hub.register <- client1
	hub.register <- client2
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.rooms[outletID]) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(hub.rooms[outletID]))
	}
	hub.mu.RUnlock()

	// Unregister first client
	hub.unregister <- client1
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.rooms[outletID]) != 1 {
		t.Fatalf("expected 1 client after first unregister, got %d", len(hub.rooms[outletID]))
	}
	hub.mu.RUnlock()

	// Unregister second client
	hub.unregister <- client2
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if hub.rooms[outletID] != nil {
		t.Fatal("room should be deleted when last client unregisters")
	}
	hub.mu.RUnlock()
}

func TestBroadcastToNonExistentOutlet(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a client for outlet1
	outlet1 := uuid.New()
	client1 := mockClient(hub, outlet1)
	hub.register <- client1
	time.Sleep(10 * time.Millisecond)

	// Broadcast to outlet2 (doesn't exist)
	outlet2 := uuid.New()
	event := Event{
		Type:    "order.created",
		Payload: json.RawMessage(`{"test":"data"}`),
	}
	hub.BroadcastToOutlet(outlet2, event)

	// client1 should NOT receive anything
	select {
	case <-client1.send:
		t.Fatal("client should not receive message for different outlet")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message
	}
}
