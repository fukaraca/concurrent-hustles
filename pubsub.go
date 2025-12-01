// Question 6: Publisher-Subscriber Pattern
// Implement a thread-safe pub/sub system

package main

import (
	"fmt"
	"sync"
	"time"
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
		return
	}
	subs := append([]chan Message(nil), ps.subscribers[msg.Topic]...)
	ps.mu.RUnlock()
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

func main() {
	ps := NewPubSub()
	defer ps.Close()

	var wg sync.WaitGroup

	// Subscriber 1: sports
	wg.Add(1)
	sub1 := ps.Subscribe("sports")
	go func() {
		defer wg.Done()
		for msg := range sub1 {
			fmt.Printf("[Sub1] Received: %s - %s\n", msg.Topic, msg.Content)
		}
	}()

	// Subscriber 2: sports and news
	wg.Add(1)
	sub2Sports := ps.Subscribe("sports")
	sub2News := ps.Subscribe("news")
	go func() {
		defer wg.Done()
		for {
			select {
			case msg, ok := <-sub2Sports:
				if !ok {
					return
				}
				fmt.Printf("[Sub2] Received: %s - %s\n", msg.Topic, msg.Content)
			case msg, ok := <-sub2News:
				if !ok {
					return
				}
				fmt.Printf("[Sub2] Received: %s - %s\n", msg.Topic, msg.Content)
			}
		}
	}()

	// Give subscribers time to set up
	time.Sleep(100 * time.Millisecond)

	// Publish messages
	ps.Publish(Message{Topic: "sports", Content: "Team A wins!"})
	ps.Publish(Message{Topic: "news", Content: "Breaking news!"})
	ps.Publish(Message{Topic: "sports", Content: "Player traded1"})
	ps.Publish(Message{Topic: "sports", Content: "Player traded2"})
	ps.Publish(Message{Topic: "sports", Content: "Player traded3"})
	ps.Publish(Message{Topic: "sports", Content: "Player traded4"})
	ps.Publish(Message{Topic: "sports", Content: "Player traded5"})
	time.Sleep(200 * time.Millisecond)

	ps.Close()

	wg.Wait()
}

// Test: Run with `go run main.go` and `go test -race`
// Expected: Sub1 receives 2 sports messages, Sub2 receives all 3 messages
// No race conditions, no deadlocks, clean shutdown
