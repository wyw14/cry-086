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
	cursorA    fleet.Pose
	cursorB    fleet.Pose
}

type simulationPlan struct {
	relations  []fleet.Relation
	poses      map[string]fleet.Pose
	velocities map[string][2]int64
}

func New(repository Repository, c clock.Clock) *Service {
	return &Service{repository: repository, clock: c, steps: 12, interval: time.Second}
}

func (s *Service) Simulate(ctx context.Context, siteID string, velocities map[string][2]int64) ([]fleet.CollisionRisk, error) {
	plan, err := s.preparePlan(ctx, siteID, velocities)
	if err != nil {
		return nil, err
	}
	if len(plan.relations) == 0 {
		return []fleet.CollisionRisk{}, nil
	}
	results, err := s.executePlan(ctx, plan)
	if err != nil {
		return nil, err
	}
	return s.persistResults(ctx, results)
}

func (s *Service) preparePlan(ctx context.Context, siteID string, velocities map[string][2]int64) (simulationPlan, error) {
	relations, err := s.repository.Relations(ctx, siteID)
	if err != nil {
		return simulationPlan{}, err
	}
	poses := make(map[string]fleet.Pose)
	for _, relation := range relations {
		for _, craneID := range []string{relation.CraneAID, relation.CraneBID} {
			if _, exists := poses[craneID]; exists {
				continue
			}
			pose, err := s.repository.LatestPose(ctx, craneID)
			if err != nil {
				return simulationPlan{}, err
			}
			poses[craneID] = pose
		}
	}
	return simulationPlan{relations: relations, poses: poses, velocities: velocities}, nil
}

func (s *Service) executePlan(ctx context.Context, plan simulationPlan) ([]fleet.CollisionRisk, error) {
	results := make(chan fleet.CollisionRisk, len(plan.relations))
	errorsChannel := make(chan error, len(plan.relations))
	var workers sync.WaitGroup
	for _, relation := range plan.relations {
		relation := relation
		workers.Add(1)
		go func() {
			defer workers.Done()
			risk, found, err := s.simulateRelation(ctx, plan, relation)
			if err != nil {
				errorsChannel <- err
				return
			}
			if found {
				results <- risk
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
		risks = append(risks, risk)
	}
	sort.Slice(risks, func(i, j int) bool { return risks[i].PredictedAt.Before(risks[j].PredictedAt) })
	return risks, nil
}

func (s *Service) simulateRelation(ctx context.Context, plan simulationPlan, relation fleet.Relation) (fleet.CollisionRisk, bool, error) {
	s.cursorA = plan.poses[relation.CraneAID]
	s.cursorB = plan.poses[relation.CraneBID]
	for step := 1; step <= s.steps; step++ {
		if err := ctx.Err(); err != nil {
			return fleet.CollisionRisk{}, false, err
		}
		velocityA := plan.velocities[s.cursorA.CraneID]
		velocityB := plan.velocities[s.cursorB.CraneID]
		s.cursorA = fleet.Project(s.cursorA, velocityA[0], velocityA[1], s.interval)
		s.cursorB = fleet.Project(s.cursorB, velocityB[0], velocityB[1], s.interval)
		risk := fleet.EvaluateRelation(relation, s.cursorA, s.cursorB, s.clock.Now().Add(time.Duration(step)*s.interval))
		if risk.Critical {
			return risk, true, nil
		}
	}
	return fleet.CollisionRisk{}, false, nil
}

func (s *Service) persistResults(ctx context.Context, risks []fleet.CollisionRisk) ([]fleet.CollisionRisk, error) {
	for _, risk := range risks {
		if err := s.repository.StoreCollisionRisk(ctx, risk); err != nil {
			return nil, errors.New("collision risk could not be stored")
		}
	}
	return risks, nil
}
