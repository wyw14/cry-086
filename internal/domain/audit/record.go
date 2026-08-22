package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Record struct {
	ID           string         `json:"id"`
	SiteID       string         `json:"site_id"`
	ActorID      string         `json:"actor_id"`
	Source       string         `json:"source"`
	Action       string         `json:"action"`
	Resource     string         `json:"resource"`
	ResourceID   string         `json:"resource_id"`
	Before       map[string]any `json:"before,omitempty"`
	After        map[string]any `json:"after,omitempty"`
	Reason       string         `json:"reason"`
	RequestID    string         `json:"request_id"`
	OccurredAt   time.Time      `json:"occurred_at"`
	PreviousHash string         `json:"previous_hash,omitempty"`
	Hash         string         `json:"hash"`
}

func New(id, siteID, actorID, source, action, resource, resourceID, reason, requestID string, before, after map[string]any, at time.Time, previousHash string) Record {
	record := Record{ID: id, SiteID: siteID, ActorID: actorID, Source: source, Action: action, Resource: resource, ResourceID: resourceID, Before: clone(before), After: clone(after), Reason: reason, RequestID: requestID, OccurredAt: at.UTC(), PreviousHash: previousHash}
	record.Hash = record.calculateHash()
	return record
}

func (r Record) Verify() bool {
	return r.Hash != "" && r.Hash == r.calculateHash()
}

func (r Record) calculateHash() string {
	payload := struct {
		ID, SiteID, ActorID, Source, Action, Resource, ResourceID, Reason, RequestID, PreviousHash string
		Before, After                                                                              map[string]any
		OccurredAt                                                                                 time.Time
	}{r.ID, r.SiteID, r.ActorID, r.Source, r.Action, r.Resource, r.ResourceID, r.Reason, r.RequestID, r.PreviousHash, r.Before, r.After, r.OccurredAt}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func clone(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
