package eventbus

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// 事件类型常量
const (
	EventTerminalOutput   = "terminal.output"
	EventTerminalStatus   = "terminal.status"
	EventApprovalRequired = "approval.required"
	EventTaskUpdated      = "task.updated"
)

type Handler func(Event)

type subscription struct {
	id      string
	ch      chan Event
	handler Handler
}

// MemoryBus 内存事件总线（非持久化，进程内有效）。
type MemoryBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[string]*subscription // eventType -> handlerID -> subscription
	bufferSize  int
}

func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		subscribers: make(map[string]map[string]*subscription),
		bufferSize:  256,
	}
}

func (b *MemoryBus) Subscribe(eventType string, handler Handler) string {
	if b == nil || eventType == "" || handler == nil {
		return ""
	}

	sub := &subscription{
		id:      uuid.New().String(),
		ch:      make(chan Event, b.bufferSize),
		handler: handler,
	}

	b.mu.Lock()
	if _, ok := b.subscribers[eventType]; !ok {
		b.subscribers[eventType] = make(map[string]*subscription)
	}
	b.subscribers[eventType][sub.id] = sub
	b.mu.Unlock()

	go func() {
		for event := range sub.ch {
			func() {
				defer func() {
					_ = recover()
				}()
				sub.handler(event)
			}()
		}
	}()

	return sub.id
}

func (b *MemoryBus) Publish(event Event) {
	if b == nil || event.Type == "" {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers[event.Type] {
		select {
		case sub.ch <- event:
		default:
			// 通道已满，丢弃事件
		}
	}
}

func (b *MemoryBus) Unsubscribe(eventType, handlerID string) {
	if b == nil || eventType == "" || handlerID == "" {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	typeSubs, ok := b.subscribers[eventType]
	if !ok {
		return
	}

	sub, ok := typeSubs[handlerID]
	if !ok {
		return
	}

	close(sub.ch)
	delete(typeSubs, handlerID)

	if len(typeSubs) == 0 {
		delete(b.subscribers, eventType)
	}
}
