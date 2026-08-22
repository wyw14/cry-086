package outbox

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

type Message struct {
	ID          string
	Topic       string
	Key         string
	Payload     []byte
	Attempts    int
	MaxAttempts int
	AvailableAt time.Time
	CreatedAt   time.Time
	LastError   string
}

type Publisher interface {
	Publish(context.Context, Message) error
}

type Queue struct {
	mu       sync.Mutex
	pending  []Message
	dead     []Message
	baseWait time.Duration
}

func NewQueue(baseWait time.Duration) *Queue {
	return &Queue{baseWait: baseWait}
}

func (q *Queue) Add(message Message) error {
	if message.ID == "" || message.Topic == "" || message.MaxAttempts < 1 {
		return errors.New("invalid outbox message")
	}
	q.mu.Lock()
	q.pending = append(q.pending, message)
	q.mu.Unlock()
	return nil
}

func (q *Queue) Dispatch(ctx context.Context, now time.Time, publisher Publisher) (int, error) {
	q.mu.Lock()
	due := make([]Message, 0)
	remaining := make([]Message, 0, len(q.pending))
	for _, message := range q.pending {
		if !message.AvailableAt.After(now) {
			due = append(due, message)
		} else {
			remaining = append(remaining, message)
		}
	}
	q.pending = remaining
	q.mu.Unlock()
	completed := 0
	for _, message := range due {
		if err := ctx.Err(); err != nil {
			q.requeue(message)
			return completed, err
		}
		if err := publisher.Publish(ctx, message); err != nil {
			message.Attempts++
			message.LastError = err.Error()
			if message.Attempts >= message.MaxAttempts {
				q.mu.Lock()
				q.dead = append(q.dead, message)
				q.mu.Unlock()
			} else {
				multiplier := time.Duration(math.Pow(2, float64(message.Attempts-1)))
				message.AvailableAt = now.Add(q.baseWait * multiplier)
				q.requeue(message)
			}
			continue
		}
		completed++
	}
	return completed, nil
}

func (q *Queue) requeue(message Message) {
	q.mu.Lock()
	q.pending = append(q.pending, message)
	q.mu.Unlock()
}

func (q *Queue) DeadLetters() []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]Message, len(q.dead))
	copy(result, q.dead)
	return result
}
