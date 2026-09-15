// Package hub broadcasts named server-sent events to connected pages.
package hub

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

const pingInterval = 15 * time.Second

// Hub is an in-process server-sent event broadcaster.
type Hub struct {
	mu          sync.Mutex
	subscribers map[chan string]struct{}
}

// New creates an empty Hub.
func New() *Hub {
	return &Hub{subscribers: make(map[chan string]struct{})}
}

// Publish sends a named event to each connected page. A slow page may miss an
// event because every event tells clients to fetch the current server state.
func (h *Hub) Publish(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for subscriber := range h.subscribers {
		select {
		case subscriber <- name:
		default:
		}
	}
}

// ServeHTTP keeps an SSE connection open until the request is canceled.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming is unavailable.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subscriber := make(chan string, 1)
	h.subscribe(subscriber)
	defer h.unsubscribe(subscriber)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	pings := time.NewTicker(pingInterval)
	defer pings.Stop()

	for {
		select {
		case name := <-subscriber:
			_, _ = fmt.Fprintf(w, "event: %s\ndata: update\n\n", name)
			flusher.Flush()
		case <-pings.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Hub) subscribe(subscriber chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[subscriber] = struct{}{}
}

func (h *Hub) unsubscribe(subscriber chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers, subscriber)
}
