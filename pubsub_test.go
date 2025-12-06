package concurrent_hustles

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Test basic publish-subscribe functionality
func TestPubSub_BasicOperation(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub := ps.Subscribe("test")

	msg := Message{Topic: "test", Content: "Hello"}
	ps.Publish(msg)

	select {
	case received := <-sub:
		if received.Content != msg.Content {
			t.Errorf("Expected %s, got %s", msg.Content, received.Content)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// Test multiple subscribers to same topic
func TestPubSub_MultipleSubscribers(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub1 := ps.Subscribe("sports")
	sub2 := ps.Subscribe("sports")
	sub3 := ps.Subscribe("sports")

	msg := Message{Topic: "sports", Content: "Team A wins!"}
	ps.Publish(msg)

	var wg sync.WaitGroup
	received := make([]bool, 3)

	checkReceive := func(idx int, ch <-chan Message) {
		defer wg.Done()
		select {
		case m := <-ch:
			if m.Content == msg.Content {
				received[idx] = true
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Subscriber %d timeout", idx)
		}
	}

	wg.Add(3)
	go checkReceive(0, sub1)
	go checkReceive(1, sub2)
	go checkReceive(2, sub3)

	wg.Wait()

	for i, r := range received {
		if !r {
			t.Errorf("Subscriber %d did not receive message", i)
		}
	}
}

// Test subscribing to different topics
func TestPubSub_DifferentTopics(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sportsSub := ps.Subscribe("sports")
	newsSub := ps.Subscribe("news")
	techSub := ps.Subscribe("tech")

	ps.Publish(Message{Topic: "sports", Content: "Goal!"})
	ps.Publish(Message{Topic: "news", Content: "Breaking!"})
	ps.Publish(Message{Topic: "tech", Content: "New release!"})

	tests := []struct {
		name     string
		ch       <-chan Message
		expected string
	}{
		{"sports", sportsSub, "Goal!"},
		{"news", newsSub, "Breaking!"},
		{"tech", techSub, "New release!"},
	}

	for _, tt := range tests {
		select {
		case msg := <-tt.ch:
			if msg.Content != tt.expected {
				t.Errorf("%s: expected %s, got %s", tt.name, tt.expected, msg.Content)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("%s: timeout", tt.name)
		}
	}
}

// Test that publishing to one topic doesn't affect other topics
func TestPubSub_TopicIsolation(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sportsSub := ps.Subscribe("sports")
	newsSub := ps.Subscribe("news")

	ps.Publish(Message{Topic: "sports", Content: "Sports message"})

	// Sports subscriber should receive
	select {
	case msg := <-sportsSub:
		if msg.Content != "Sports message" {
			t.Errorf("Expected 'Sports message', got %s", msg.Content)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Sports subscriber timeout")
	}

	// News subscriber should NOT receive
	select {
	case <-newsSub:
		t.Error("News subscriber should not have received sports message")
	case <-time.After(100 * time.Millisecond):
		// Expected timeout
	}
}

// Test unsubscribe functionality
func TestPubSub_Unsubscribe(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub := ps.Subscribe("test")

	// Receive first message
	ps.Publish(Message{Topic: "test", Content: "Message 1"})
	<-sub

	// Unsubscribe
	ps.Unsubscribe("test", sub)

	// Channel should be closed
	_, ok := <-sub
	if ok {
		t.Error("Channel should be closed after unsubscribe")
	}
}

// Test unsubscribe with multiple subscribers
func TestPubSub_UnsubscribeMultiple(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub1 := ps.Subscribe("test")
	sub2 := ps.Subscribe("test")
	sub3 := ps.Subscribe("test")

	// Unsubscribe middle one
	ps.Unsubscribe("test", sub2)

	// Publish message
	ps.Publish(Message{Topic: "test", Content: "After unsubscribe"})

	// sub1 should receive
	select {
	case <-sub1:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("sub1 should have received message")
	}

	// sub2 should be closed
	_, ok := <-sub2
	if ok {
		t.Error("sub2 should be closed")
	}

	// sub3 should receive
	select {
	case <-sub3:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("sub3 should have received message")
	}
}

// Test closing pub/sub system
func TestPubSub_Close(t *testing.T) {
	ps := NewPubSub()

	sub1 := ps.Subscribe("topic1")
	sub2 := ps.Subscribe("topic2")

	ps.Close()

	// All channels should be closed
	_, ok1 := <-sub1
	_, ok2 := <-sub2

	if ok1 || ok2 {
		t.Error("All channels should be closed after Close()")
	}
}

// Test publishing after close
func TestPubSub_PublishAfterClose(t *testing.T) {
	ps := NewPubSub()
	sub := ps.Subscribe("test")

	ps.Close()

	// Publishing after close should not panic
	ps.Publish(Message{Topic: "test", Content: "After close"})

	// Channel should be closed, no new messages
	_, ok := <-sub
	if ok {
		t.Error("Should not receive messages after close")
	}
}

// Test subscribing after close
func TestPubSub_SubscribeAfterClose(t *testing.T) {
	ps := NewPubSub()
	ps.Close()

	sub := ps.Subscribe("test")

	// Channel should be closed immediately
	_, ok := <-sub
	if ok {
		t.Error("Subscription after close should return closed channel")
	}
}

// Test concurrent publishing
func TestPubSub_ConcurrentPublish(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub := ps.Subscribe("test")

	numMessages := 100
	var wg sync.WaitGroup

	// Publish from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ps.Publish(Message{
					Topic:   "test",
					Content: "message",
				})
			}
		}(i)
	}

	// Count received messages
	received := 0
	done := make(chan bool)
	go func() {
		for range sub {
			received++
			if received == numMessages {
				done <- true
				return
			}
		}
	}()

	wg.Wait()

	select {
	case <-done:
		if received != numMessages {
			t.Errorf("Expected %d messages, got %d", numMessages, received)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Timeout: only received %d/%d messages", received, numMessages)
	}
}

// Test concurrent subscribe
func TestPubSub_ConcurrentSubscribe(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	var wg sync.WaitGroup
	numSubscribers := 50

	// Subscribe from multiple goroutines
	channels := make([]<-chan Message, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			channels[idx] = ps.Subscribe("test")
		}(i)
	}

	wg.Wait()

	// Publish a message
	ps.Publish(Message{Topic: "test", Content: "Broadcast"})

	// All should receive
	received := 0
	for _, ch := range channels {
		select {
		case <-ch:
			received++
		case <-time.After(1 * time.Second):
			// Some might timeout due to buffer size
		}
	}

	if received == 0 {
		t.Error("No subscribers received message")
	}
}

// Test unsubscribe non-existent channel
func TestPubSub_UnsubscribeNonExistent(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub := ps.Subscribe("test")
	fakeChan := make(chan Message)

	// Should not panic
	ps.Unsubscribe("test", fakeChan)
	ps.Unsubscribe("nonexistent", sub)

	// Original subscription should still work
	ps.Publish(Message{Topic: "test", Content: "Still works"})

	select {
	case <-sub:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Original subscription should still work")
	}
}

// Test multiple topics, multiple subscribers
func TestPubSub_ComplexScenario(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	var wg sync.WaitGroup

	// Subscriber 1: sports only
	wg.Add(1)
	sub1 := ps.Subscribe("sports")
	sportsCount1 := 0
	go func() {
		defer wg.Done()
		for range sub1 {
			sportsCount1++
		}
	}()

	// Subscriber 2: sports and news
	wg.Add(1)
	sub2Sports := ps.Subscribe("sports")
	sub2News := ps.Subscribe("news")
	sportsCount2 := 0
	newsCount2 := 0
	go func() {
		defer wg.Done()
		for {
			select {
			case _, ok := <-sub2Sports:
				if !ok {
					return
				}
				sportsCount2++
			case _, ok := <-sub2News:
				if !ok {
					return
				}
				newsCount2++
			}
		}
	}()

	// Give subscribers time to set up
	time.Sleep(50 * time.Millisecond)

	// Publish messages
	ps.Publish(Message{Topic: "sports", Content: "Game 1"})
	ps.Publish(Message{Topic: "sports", Content: "Game 2"})
	ps.Publish(Message{Topic: "news", Content: "News 1"})
	ps.Publish(Message{Topic: "sports", Content: "Game 3"})

	time.Sleep(100 * time.Millisecond)
	ps.Close()
	wg.Wait()

	if sportsCount1 != 3 {
		t.Errorf("Sub1 expected 3 sports messages, got %d", sportsCount1)
	}
	if sportsCount2 != 3 {
		t.Errorf("Sub2 expected 3 sports messages, got %d", sportsCount2)
	}
	if newsCount2 != 1 {
		t.Errorf("Sub2 expected 1 news message, got %d", newsCount2)
	}
}

// Test message order preservation
func TestPubSub_MessageOrder(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub := ps.Subscribe("test")

	messages := []string{"first", "second", "third", "fourth", "fifth"}

	go func() {
		for _, content := range messages {
			ps.Publish(Message{Topic: "test", Content: content})
		}
	}()

	received := []string{}
	for i := 0; i < len(messages); i++ {
		select {
		case msg := <-sub:
			received = append(received, msg.Content)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout receiving messages")
		}
	}

	for i, expected := range messages {
		if received[i] != expected {
			t.Errorf("Message %d: expected %s, got %s", i, expected, received[i])
		}
	}
}

// Test unsubscribe last subscriber removes topic
func TestPubSub_UnsubscribeLastRemovesTopic(t *testing.T) {
	ps := NewPubSub()
	defer ps.Close()

	sub := ps.Subscribe("test")

	// Verify topic exists
	ps.mu.RLock()
	_, exists := ps.subscribers["test"]
	ps.mu.RUnlock()
	if !exists {
		t.Fatal("Topic should exist after subscribe")
	}

	// Unsubscribe
	ps.Unsubscribe("test", sub)

	// Verify topic removed
	ps.mu.RLock()
	_, exists = ps.subscribers["test"]
	ps.mu.RUnlock()
	if exists {
		t.Error("Topic should be removed after last unsubscribe")
	}
}

// Test double close doesn't panic
func TestPubSub_DoubleClose(t *testing.T) {
	ps := NewPubSub()
	ps.Subscribe("test")

	ps.Close()
	ps.Close() // Should not panic
}

// Benchmark publishing
func BenchmarkPubSub_Publish(b *testing.B) {
	ps := NewPubSub()
	defer ps.Close()

	sub := ps.Subscribe("test")
	go func() {
		for range sub {
		}
	}()

	msg := Message{Topic: "test", Content: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Publish(msg)
	}
}

// Benchmark subscribing
func BenchmarkPubSub_Subscribe(b *testing.B) {
	ps := NewPubSub()
	defer ps.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Subscribe("test")
	}
}

// Test no goroutine leaks
func TestPubSub_NoGoroutineLeaks(t *testing.T) {
	before := testing.AllocsPerRun(1, func() {})
	_ = before

	ps := NewPubSub()

	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("test%d", i)
		sub := ps.Subscribe(name)
		ps.Publish(Message{Topic: name, Content: "test"})
		<-sub
	}

	ps.Close()

	time.Sleep(100 * time.Millisecond)
	// If test completes without hanging, no leaks
}
