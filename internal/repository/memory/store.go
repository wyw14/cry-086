package memory

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wyw14/cry-086/internal/domain/alarm"
	"github.com/wyw14/cry-086/internal/domain/audit"
	"github.com/wyw14/cry-086/internal/domain/fleet"
	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/maintenance"
	"github.com/wyw14/cry-086/internal/domain/report"
	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
	"github.com/wyw14/cry-086/internal/platform/files"
)

var ErrNotFound = errors.New("record not found")

type quarantinedReading struct {
	Reading telemetry.RawReading
	Reason  string
}

type Store struct {
	mu sync.RWMutex

	sites          map[string]site.Site
	cranes         map[string]site.TowerCrane
	sensors        map[string]sensor.Sensor
	calibrations   map[string]sensor.Calibration
	readings       map[string][]telemetry.NormalizedReading
	events         map[string]bool
	lastSequences  map[string]int64
	quarantined    []quarantinedReading
	configs        map[string]safety.Config
	decisions      map[string][]safety.Decision
	alarms         map[string]alarm.Event
	alarmByEvent   map[string]string
	clearSince     map[string]time.Time
	audits         []audit.Record
	relations      map[string][]fleet.Relation
	poses          map[string]fleet.Pose
	collisionRisks []fleet.CollisionRisk
	workOrders     map[string]maintenance.WorkOrder
	stopRecords    map[string]maintenance.StopRecord
	users          map[string]identity.User
	usernames      map[string]string
	refreshTokens  map[string]identity.RefreshToken
	reports        map[string][]report.RegulatoryReport
	timeline       map[string][]report.TimelineEvent
	files          map[string]files.Metadata
}

func New() *Store {
	return &Store{
		sites:         make(map[string]site.Site),
		cranes:        make(map[string]site.TowerCrane),
		sensors:       make(map[string]sensor.Sensor),
		calibrations:  make(map[string]sensor.Calibration),
		readings:      make(map[string][]telemetry.NormalizedReading),
		events:        make(map[string]bool),
		lastSequences: make(map[string]int64),
		configs:       make(map[string]safety.Config),
		decisions:     make(map[string][]safety.Decision),
		alarms:        make(map[string]alarm.Event),
		alarmByEvent:  make(map[string]string),
		clearSince:    make(map[string]time.Time),
		relations:     make(map[string][]fleet.Relation),
		poses:         make(map[string]fleet.Pose),
		workOrders:    make(map[string]maintenance.WorkOrder),
		stopRecords:   make(map[string]maintenance.StopRecord),
		users:         make(map[string]identity.User),
		usernames:     make(map[string]string),
		refreshTokens: make(map[string]identity.RefreshToken),
		reports:       make(map[string][]report.RegulatoryReport),
		timeline:      make(map[string][]report.TimelineEvent),
		files:         make(map[string]files.Metadata),
	}
}

func calibrationKey(id string, version int64) string {
	return fmt.Sprintf("%s:%d", id, version)
}

func configKey(id string, version int64) string {
	return fmt.Sprintf("%s:%d", id, version)
}

func eventKey(craneID, eventID string) string {
	return craneID + ":" + eventID
}
