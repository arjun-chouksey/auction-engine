package realtime

import "sync"

type Message struct {
	RFQID  string `json:"rfq_id"`
	Reason string `json:"reason"`
}

type Hub struct {
	mu          sync.Mutex
	bufferSize  int
	subscribers map[string]map[chan Message]struct{}
}

func NewHub(bufferSize int) *Hub {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &Hub{bufferSize: bufferSize, subscribers: make(map[string]map[chan Message]struct{})}
}

func (hub *Hub) Subscribe(rfqID string) (<-chan Message, func()) {
	hub.mu.Lock()
	channel := make(chan Message, hub.bufferSize)
	if hub.subscribers[rfqID] == nil {
		hub.subscribers[rfqID] = make(map[chan Message]struct{})
	}
	hub.subscribers[rfqID][channel] = struct{}{}
	hub.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			hub.mu.Lock()
			defer hub.mu.Unlock()
			if subscribers := hub.subscribers[rfqID]; subscribers != nil {
				if _, exists := subscribers[channel]; exists {
					delete(subscribers, channel)
					close(channel)
				}
				if len(subscribers) == 0 {
					delete(hub.subscribers, rfqID)
				}
			}
		})
	}
	return channel, cancel
}

func (hub *Hub) Publish(rfqID, reason string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	subscribers := hub.subscribers[rfqID]
	message := Message{RFQID: rfqID, Reason: reason}
	for channel := range subscribers {
		select {
		case channel <- message:
		default:
			delete(subscribers, channel)
			close(channel)
		}
	}
	if len(subscribers) == 0 {
		delete(hub.subscribers, rfqID)
	}
}

func (hub *Hub) SubscriberCount(rfqID string) int {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.subscribers[rfqID])
}
