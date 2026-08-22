package memory

import (
	"context"
	"errors"

	"github.com/wyw14/cry-086/internal/domain/fleet"
	"github.com/wyw14/cry-086/internal/domain/maintenance"
)

func (s *Store) SeedRelation(siteID string, value fleet.Relation) {
	s.mu.Lock()
	s.relations[siteID] = append(s.relations[siteID], value)
	s.mu.Unlock()
}

func (s *Store) SeedPose(value fleet.Pose) {
	s.mu.Lock()
	s.poses[value.CraneID] = value
	s.mu.Unlock()
}

func (s *Store) Relations(ctx context.Context, siteID string) ([]fleet.Relation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := append([]fleet.Relation(nil), s.relations[siteID]...)
	return values, nil
}

func (s *Store) LatestPose(ctx context.Context, craneID string) (fleet.Pose, error) {
	if err := ctx.Err(); err != nil {
		return fleet.Pose{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.poses[craneID]
	if !ok {
		return fleet.Pose{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) StoreCollisionRisk(ctx context.Context, value fleet.CollisionRisk) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.collisionRisks = append(s.collisionRisks, value)
	s.mu.Unlock()
	return nil
}

func (s *Store) SeedWorkOrder(value maintenance.WorkOrder) {
	s.mu.Lock()
	s.workOrders[value.ID] = value
	s.mu.Unlock()
}

func (s *Store) FindWorkOrder(ctx context.Context, id string) (maintenance.WorkOrder, error) {
	if err := ctx.Err(); err != nil {
		return maintenance.WorkOrder{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.workOrders[id]
	if !ok {
		return maintenance.WorkOrder{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) UpdateWorkOrder(ctx context.Context, value maintenance.WorkOrder, expectedVersion int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.workOrders[value.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Version != expectedVersion || value.Version != expectedVersion+1 {
		return errors.New("work order optimistic version conflict")
	}
	s.workOrders[value.ID] = value
	return nil
}

func (s *Store) StoreStopRecord(ctx context.Context, value maintenance.StopRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.stopRecords {
		if existing.CraneID == value.CraneID && existing.ResumedAt == nil {
			return errors.New("open stop record already exists")
		}
	}
	s.stopRecords[value.ID] = value
	return nil
}

func (s *Store) FindOpenStopRecord(ctx context.Context, craneID string) (maintenance.StopRecord, error) {
	if err := ctx.Err(); err != nil {
		return maintenance.StopRecord{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, value := range s.stopRecords {
		if value.CraneID == craneID && value.ResumedAt == nil {
			return value, nil
		}
	}
	return maintenance.StopRecord{}, ErrNotFound
}

func (s *Store) UpdateStopRecord(ctx context.Context, value maintenance.StopRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.stopRecords[value.ID]
	if !ok {
		return ErrNotFound
	}
	if current.ResumedAt != nil || value.ResumedAt == nil {
		return errors.New("stop record transition is invalid")
	}
	s.stopRecords[value.ID] = value
	return nil
}
