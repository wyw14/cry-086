package idempotency

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrInProgress = errors.New("idempotent operation is already in progress")

type Record struct {
	Key        string
	Scope      string
	RequestSum string
	Response   []byte
	StatusCode int
	ExpiresAt  time.Time
	Completed  bool
}

type Store interface {
	Begin(context.Context, string, string, string, time.Time) (Record, bool, error)
	Complete(context.Context, string, string, []byte, int) error
}

type Memory struct {
	mu      sync.Mutex
	records map[string]Record
}

func NewMemory() *Memory {
	return &Memory{records: make(map[string]Record)}
}

func (m *Memory) Begin(ctx context.Context, scope, key, requestSum string, expiresAt time.Time) (Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, false, err
	}
	compound := scope + ":" + key
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.records[compound]; ok && existing.ExpiresAt.After(time.Now().UTC()) {
		if existing.RequestSum != requestSum {
			return Record{}, false, errors.New("idempotency key reused with different payload")
		}
		if !existing.Completed {
			return Record{}, false, ErrInProgress
		}
		return existing, true, nil
	}
	record := Record{Key: key, Scope: scope, RequestSum: requestSum, ExpiresAt: expiresAt.UTC()}
	m.records[compound] = record
	return record, false, nil
}

func (m *Memory) Complete(ctx context.Context, scope, key string, response []byte, status int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	compound := scope + ":" + key
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[compound]
	if !ok {
		return errors.New("idempotency record not found")
	}
	record.Response = append([]byte(nil), response...)
	record.StatusCode = status
	record.Completed = true
	m.records[compound] = record
	return nil
}
