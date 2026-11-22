package api

import "sync"

const subscriberBuffer = 32

type Hub struct {
	mu     sync.RWMutex
	topics map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{topics: make(map[string]map[chan []byte]struct{})}
}

func (h *Hub) Subscribe(topic string) (<-chan []byte, func()) {
	ch := make(chan []byte, subscriberBuffer)
	h.mu.Lock()
	subs := h.topics[topic]
	if subs == nil {
		subs = make(map[chan []byte]struct{})
		h.topics[topic] = subs
	}
	subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if subs := h.topics[topic]; subs != nil {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.topics, topic)
			}
		}
	}
}

func (h *Hub) Publish(topic string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.topics[topic] {
		select {
		case ch <- data:
		default:
		}
	}
}
