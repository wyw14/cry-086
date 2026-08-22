package outbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

type failingPublisher struct{}

func (failingPublisher) Publish(context.Context, Message) error {
	return errors.New("offline adapter failure")
}

func TestDispatchMovesExhaustedMessageToDeadLetter(t *testing.T) {
	queue := NewQueue(time.Second)
	now := time.Now().UTC()
	if err := queue.Add(Message{ID: "m", Topic: "alarm", MaxAttempts: 1, AvailableAt: now}); err != nil {
		t.Fatal(err)
	}
	if completed, err := queue.Dispatch(context.Background(), now, failingPublisher{}); err != nil || completed != 0 {
		t.Fatalf("Dispatch() = %d,%v", completed, err)
	}
	if len(queue.DeadLetters()) != 1 {
		t.Fatal("failed message was not moved to dead letter")
	}
}
