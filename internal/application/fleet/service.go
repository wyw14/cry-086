package fleetapp

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/wyw14/cry-086/internal/domain/fleet"
	"github.com/wyw14/cry-086/internal/platform/clock"
)

type Repository interface {
	Relations(context.Context, string) ([]fleet.Relation, error)
	LatestPose(context.Context, string) (fleet.Pose, error)
	StoreCollisionRisk(context.Context, fleet.CollisionRisk) error
}

type Service struct {
	repository Repository
	clock      clock.Clock
	steps      int
	interval   time.Duration
}

func New(repository Repository, c clock.Clock) *Service {
	return &Service{repository: repository, clock: c, steps: 12, interval: time.Second}
}

func (s *Service) Simulate(ctx context.Context, siteID string, velocities map[string][2]int64) ([]fleet.CollisionRisk, error) {
	relations, err := s.repository.Relations(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if len(relations) == 0 {
		return []fleet.CollisionRisk{}, nil
	}
	poses := make(map[string]fleet.Pose)
	for _, relation := range relations {
		for _, craneID := range []string{relation.CraneAID, relation.CraneBID} {
			if _, exists := poses[craneID]; exists {
				continue
			}
			pose, err := s.repository.LatestPose(ctx, craneID)
			if err != nil {
				return nil, err
			}
			poses[craneID] = pose
		}
	}
	results := make(chan fleet.CollisionRisk, len(relations)*s.steps)
	errorsChannel := make(chan error, len(relations))
	var workers sync.WaitGroup
	for _, relation := range relations {
		relation := relation
		workers.Add(1)
		go func() {
			defer workers.Done()
			a := poses[relation.CraneAID]
			b := poses[relation.CraneBID]
			for step := 1; step <= s.steps; step++ {
				if err := ctx.Err(); err != nil {
					errorsChannel <- err
					return
				}
				av := velocities[a.CraneID]
				bv := velocities[b.CraneID]
				a = fleet.Project(a, av[0], av[1], s.interval)
				b = fleet.Project(b, bv[0], bv[1], s.interval)
				risk := fleet.EvaluateRelation(relation, a, b, s.clock.Now().Add(time.Duration(step)*s.interval))
				if risk.Critical {
					results <- risk
					return
				}
			}
		}()
	}
	workers.Wait()
	close(results)
	close(errorsChannel)
	for workerErr := range errorsChannel {
		if workerErr != nil {
			return nil, workerErr
		}
	}
	risks := make([]fleet.CollisionRisk, 0)
	for risk := range results {
		if err := s.repository.StoreCollisionRisk(ctx, risk); err != nil {
			return nil, errors.New("collision risk could not be stored")
		}
		risks = append(risks, risk)
	}
	sort.Slice(risks, func(i, j int) bool { return risks[i].PredictedAt.Before(risks[j].PredictedAt) })
	return risks, nil
}
