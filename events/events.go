package events

import (
	"strings"
	"sync"
)

type Listener chan Event

// Event represents an Event sent over a Bus.
type Event struct {
	Topic string
	Data  interface{}
}

// Bus represents an Event Bus.
type Bus struct {
	listenersMx sync.Mutex
	listeners   map[string][]Listener
}

// NewBus returns a new empty Event Bus.
func NewBus() *Bus {
	return &Bus{
		listeners: make(map[string][]Listener),
	}
}

// Off unregisters a listener from the specified topics on the Bus.
func (b *Bus) Off(listener Listener, topics ...string) {
	b.listenersMx.Lock()
	defer b.listenersMx.Unlock()

	for _, topic := range topics {
		b.off(topic, listener)
	}
}

func (b *Bus) off(topic string, listener Listener) bool {
	listeners, ok := b.listeners[topic]
	if !ok {
		return false
	}
	for i, l := range listeners {
		if l != listener {
			continue
		}

		listeners = append(listeners[:i], listeners[i+1:]...)
		b.listeners[topic] = listeners
		return true
	}
	return false
}

// On registers a listener to the specified topics on the Bus.
func (b *Bus) On(listener Listener, topics ...string) {
	b.listenersMx.Lock()
	defer b.listenersMx.Unlock()

	for _, topic := range topics {
		b.on(topic, listener)
	}
}

func (b *Bus) on(topic string, listener Listener) {
	listeners, ok := b.listeners[topic]
	if !ok {
		b.listeners[topic] = []Listener{listener}
	} else {
		b.listeners[topic] = append(listeners, listener)
	}
}

// Publish publishes a message to the Bus.
func (b *Bus) Publish(topic string, data interface{}) {
	// Some of our topics for the socket support passing a more specific namespace,
	// such as "backup completed:1234" to indicate which specific backup was completed.
	//
	// In these cases, we still need to send the event using the standard listener
	// name of "backup completed".
	if strings.Contains(topic, ":") {
		parts := strings.SplitN(topic, ":", 2)

		if len(parts) == 2 {
			topic = parts[0]
		}
	}

	b.listenersMx.Lock()
	defer b.listenersMx.Unlock()

	listeners, ok := b.listeners[topic]
	if !ok {
		return
	}
	if len(listeners) < 1 {
		return
	}

	var wg sync.WaitGroup
	event := Event{Topic: topic, Data: data}
	for _, listener := range listeners {
		l := listener
		wg.Add(1)
		go func(l Listener, event Event) {
			defer wg.Done()
			l <- event
		}(l, event)
	}
	wg.Wait()
}

// Destroy destroys the Event Bus by unregistering and closing all listeners.
func (b *Bus) Destroy() {
	b.listenersMx.Lock()
	defer b.listenersMx.Unlock()

	for _, listeners := range b.listeners {
		for _, listener := range listeners {
			close(listener)
		}
	}

	b.listeners = make(map[string][]Listener)
}
