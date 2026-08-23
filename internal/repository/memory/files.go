package memory

import (
	"context"
	"errors"

	"github.com/wyw14/cry-086/internal/platform/files"
)

func (s *Store) RegisterEvidence(ctx context.Context, metadata files.Metadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if metadata.ID == "" || metadata.SiteID == "" || metadata.Checksum == "" {
		return errors.New("evidence metadata is incomplete")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.files[metadata.ID]; exists {
		return errors.New("evidence identifier already exists")
	}
	for _, existing := range s.files {
		if existing.SiteID == metadata.SiteID && existing.Checksum == metadata.Checksum {
			return errors.New("evidence content already exists for site")
		}
	}
	s.files[metadata.ID] = metadata
	return nil
}

func (s *Store) FindSiteEvidence(ctx context.Context, siteID, evidenceID string) (files.Metadata, error) {
	if err := ctx.Err(); err != nil {
		return files.Metadata{}, err
	}
	s.mu.RLock()
	metadata, exists := s.files[evidenceID]
	s.mu.RUnlock()
	if !exists || metadata.SiteID != siteID {
		return files.Metadata{}, ErrNotFound
	}
	return metadata, nil
}
