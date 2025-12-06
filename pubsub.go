// Publisher-Subscriber Pattern
// Implement a thread-safe pub/sub system

package concurrent_hustles

import (
	"sync"
)

type Message struct {
	Topic   string
	Content string
}

type PubSub struct {
	mu          sync.RWMutex
	subscribers map[string][]chan Message
	closed      bool
}

// Create a new pub/sub system
func NewPubSub() *PubSub {
	subs := make(map[string][]chan Message)
	return &PubSub{subscribers: subs}
}

// Subscribe to a topic and return a channel to receive messages
// The channel should have a buffer to prevent blocking publishers
func (ps *PubSub) Subscribe(topic string) <-chan Message {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	c := make(chan Message, 1)
	if ps.closed {
		close(c)
		return c
	}
	if v, ok := ps.subscribers[topic]; ok {
		v = append(v, c)
		ps.subscribers[topic] = v
	} else {
		ps.subscribers[topic] = []chan Message{c}
	}
	return c
}

// Publish a message to all subscribers of a topic
// Must not block if a subscriber is slow
// Should handle closed channels gracefully
func (ps *PubSub) Publish(msg Message) {
	ps.mu.RLock()
	if ps.closed {
		ps.mu.RUnlock()
		return
	}
	ps.mu.RUnlock()
	subs := append([]chan Message(nil), ps.subscribers[msg.Topic]...)
	for _, c := range subs {
		select {
		case c <- msg:
			//default: // drops when buffer is full

		}
	}
}

// Remove a subscriber channel from a topic
func (ps *PubSub) Unsubscribe(topic string, ch <-chan Message) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return
	}
	chs, ok := ps.subscribers[topic]
	if !ok || len(chs) == 0 {
		return
	}
	if len(chs) == 1 {
		if chs[0] != ch {
			return
		}
		close(chs[0])
		delete(ps.subscribers, topic)
		return
	}
	for i, c := range chs {
		if c == ch {
			close(chs[i])
			chs = append(chs[:i], chs[i+1:]...) // in case i is last item what happens?
			ps.subscribers[topic] = chs
			return
		}
	}
}

// Close the pub/sub system and all subscriber channels
func (ps *PubSub) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return
	}
	for _, chs := range ps.subscribers {
		for _, c := range chs {
			close(c)
		}
	}
	ps.subscribers = make(map[string][]chan Message)
	ps.closed = true
}
