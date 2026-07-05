// Server-Sent Events (SSE) for real-time push to operator consoles.
// Replaces polling — clients connect once and receive streamed events.
// Events: case.created, case.updated, mission.eta, queue.refresh
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// sseBroker manages connected SSE clients and event broadcasting.
type sseBroker struct {
	mu      sync.RWMutex
	clients map[chan string]bool
}

var broker = &sseBroker{clients: make(map[chan string]bool)}

// subscribe adds a client channel and returns a cleanup function.
func (b *sseBroker) subscribe(ch chan string) func() {
	b.mu.Lock()
	b.clients[ch] = true
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.clients, ch)
		b.mu.Unlock()
		close(ch)
	}
}

// broadcast sends an event to all connected clients.
func (b *sseBroker) broadcast(event string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default:
			// client buffer full — skip (non-blocking)
		}
	}
}

// publishSSE is a convenience function to broadcast a typed event.
func publishSSE(eventType string, data map[string]any) {
	payload, _ := json.Marshal(data)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(payload))
	broker.broadcast(msg)
}

// handleSSE serves the SSE event stream to clients.
func handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan string, 64)
	cleanup := broker.subscribe(ch)
	defer cleanup()

	// Send initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, msg)
			flusher.Flush()
		}
	}
}

// initSSE starts the SSE broker (no-op — broker is always ready).
func initSSE() {
	log.Printf("sse: broker ready")
}
