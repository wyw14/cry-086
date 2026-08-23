package notification

import (
	"context"
	"sync"
	"time"
)

type Notice struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	Channel   string    `json:"channel"`
	Recipient string    `json:"recipient"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Sender interface {
	Send(context.Context, Notice) error
}

type LocalSender struct {
	mu      sync.Mutex
	notices []Notice
}

func (s *LocalSender) Send(ctx context.Context, notice Notice) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.notices = append(s.notices, notice)
	s.mu.Unlock()
	return nil
}

func (s *LocalSender) Notices() []Notice {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Notice, len(s.notices))
	copy(result, s.notices)
	return result
}
