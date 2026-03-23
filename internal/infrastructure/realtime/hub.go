package realtime

import (
	"context"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*client]struct{}
}

type client struct {
	userID string
	send   func(Event) error
	close  func() error
}

func NewHub() *Hub {
	return &Hub{
		clients: map[string]map[*client]struct{}{},
	}
}

func (h *Hub) PublishToUser(_ context.Context, userID string, eventType string, payload any) error {
	event := Event{Type: eventType, Payload: payload}

	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients[userID]))
	for c := range h.clients[userID] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		if err := c.send(event); err != nil {
			h.unregister(c)
			if c.close != nil {
				_ = c.close()
			}
		}
	}
	return nil
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	server := websocket.Server{
		Handshake: func(*websocket.Config, *http.Request) error { return nil },
		Handler: websocket.Handler(func(conn *websocket.Conn) {
			c := &client{
				userID: userID,
				send: func(event Event) error {
					return websocket.JSON.Send(conn, event)
				},
				close: conn.Close,
			}
			h.register(c)
			defer h.unregister(c)
			defer conn.Close()

			for {
				var ignored string
				if err := websocket.Message.Receive(conn, &ignored); err != nil {
					return
				}
			}
		}),
	}
	server.ServeHTTP(w, r)
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.userID] == nil {
		h.clients[c.userID] = map[*client]struct{}{}
	}
	h.clients[c.userID][c] = struct{}{}
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	userClients := h.clients[c.userID]
	if userClients == nil {
		return
	}
	delete(userClients, c)
	if len(userClients) == 0 {
		delete(h.clients, c.userID)
	}
}
