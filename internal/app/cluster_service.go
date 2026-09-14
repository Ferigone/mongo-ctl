package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"mongoctl/internal/mongoconf"
	"mongoctl/internal/replicaset"
	"mongoctl/internal/store"
)

// EventClusterProgress carries step-by-step progress of a grouping operation.
const EventClusterProgress = "cluster:progress"

const primaryElectionTimeout = 60 * time.Second

// ClusterService exposes replica set management to the UI.
type ClusterService struct {
	core *Core
}

// NewClusterService binds replica set operations to the application core.
func NewClusterService(core *Core) *ClusterService {
	return &ClusterService{core: core}
}

// ReplicaSetView is a stored replica set plus its live status when reachable.
type ReplicaSetView struct {
	Name        string             `json:"name"`
	MemberIDs   []string           `json:"memberIds"`
	CreatedAt   time.Time          `json:"createdAt"`
	Status      *replicaset.Status `json:"status"`
	StatusError string             `json:"statusError"`
}

// GroupRequest asks for a set of instances to be joined into a replica set.
type GroupRequest struct {
	Name        string   `json:"name"`
	InstanceIDs []string `json:"instanceIds"`

	// ClearNonSeedData wipes the data directories of every member except the
	// seed, which initial sync requires them to be empty.
	ClearNonSeedData bool `json:"clearNonSeedData"`
}

// PreflightReport tells the UI what grouping these instances would involve,
// so the restart and any data loss are shown before the user commits.
type PreflightReport struct {
	SeedName         string   `json:"seedName"`
	RequiresRestart  []string `json:"requiresRestart"`
	PopulatedNonSeed []string `json:"populatedNonSeed"`
	EvenMemberCount  bool     `json:"evenMemberCount"`
}

// ClusterProgress is one step of a grouping operation.
type ClusterProgress struct {
	Step    string `json:"step"`
	Message string `json:"message"`
	Index   int    `json:"index"`
	Total   int    `json:"total"`
	Done    bool   `json:"done"`
	Error   string `json:"error"`
}

// List returns the configured replica sets with live status where available.
func (s *ClusterService) List() []ReplicaSetView {
	state := s.core.store.State()

	views := make([]ReplicaSetView, 0, len(state.ReplicaSets))
	for _, record := range state.ReplicaSets {
		view := ReplicaSetView{
			Name:      record.Name,
			MemberIDs: record.MemberIDs,
			CreatedAt: record.CreatedAt,
		}

		if port, ok := s.firstRunningPort(record, state); ok {
			status, err := replicaset.GetStatus(s.core.context(), port)
			if err != nil {
				view.StatusError = err.Error()
			} else {
				view.Status = &status
			}
		} else {
			view.StatusError = "no member of this replica set is running"
		}

		views = append(views, view)
	}
	return views
}

// Preflight reports what grouping the given instances would require.
func (s *ClusterService) Preflight(instanceIDs []string) (PreflightReport, error) {
	records, err := s.collect(instanceIDs)
	if err != nil {
		return PreflightReport{}, err
	}

	// Both slices start empty rather than nil: a nil slice marshals to JSON null,
	// and the UI reads their length directly.
	report := PreflightReport{
		SeedName:         records[0].Name,
		EvenMemberCount:  len(records)%2 == 0,
		RequiresRestart:  []string{},
		PopulatedNonSeed: []string{},
	}

	for index, record := range records {
		// Every member restarts: mongod reads replSetName only at startup.
		report.RequiresRestart = append(report.RequiresRestart, record.Name)

		if index == 0 {
			continue
		}
		populated, err := replicaset.DataDirPopulated(record.DataDir)
		if err != nil {
			return PreflightReport{}, err
		}
		if populated {
			report.PopulatedNonSeed = append(report.PopulatedNonSeed, record.Name)
		}
	}
	return report, nil
}

// Group turns the selected instances into a replica set. Every member is
// restarted, because mongod only honours replSetName at startup.
func (s *ClusterService) Group(request GroupRequest) error {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return errors.New("replica set name is required")
	}

	records, err := s.collect(request.InstanceIDs)
	if err != nil {
		return s.fail("validate", err)
	}

	binary := s.core.Mongod()
	if binary.Path == "" {
		return s.fail("validate", ErrNoMongod)
	}

	const totalSteps = 5
	ctx := s.core.context()

	s.progress(ClusterProgress{Step: "stop", Index: 1, Total: totalSteps,
		Message: fmt.Sprintf("Stopping %d instances", len(records))})
	for _, record := range records {
		if s.core.supervisor.IsRunning(record.ID) {
			if err := s.core.supervisor.Stop(ctx, record.ID); err != nil {
				return s.fail("stop", err)
			}
		}
	}

	s.progress(ClusterProgress{Step: "configure", Index: 2, Total: totalSteps,
		Message: "Enabling replication in each configuration"})
	for index, record := range records {
		if err := enableReplication(record, name); err != nil {
			return s.fail("configure", err)
		}
		if index > 0 && request.ClearNonSeedData {
			if err := clearDataDir(record.DataDir, record.ConfigPath); err != nil {
				return s.fail("configure", err)
			}
		}
	}

	s.progress(ClusterProgress{Step: "start", Index: 3, Total: totalSteps,
		Message: "Restarting instances with replication enabled"})
	for _, record := range records {
		if err := s.core.supervisor.Start(ctx, record, binary.Path); err != nil {
			return s.fail("start", err)
		}
	}

	s.progress(ClusterProgress{Step: "initiate", Index: 4, Total: totalSteps,
		Message: "Initiating the replica set"})
	ports := make([]int, 0, len(records))
	for _, record := range records {
		ports = append(ports, record.Port)
	}
	if err := replicaset.Initiate(ctx, name, ports); err != nil {
		return s.fail("initiate", err)
	}

	s.progress(ClusterProgress{Step: "elect", Index: 5, Total: totalSteps,
		Message: "Waiting for a primary to be elected"})
	if err := replicaset.WaitForPrimary(ctx, ports[0], primaryElectionTimeout); err != nil {
		return s.fail("elect", err)
	}

	if err := s.persistGroup(name, records); err != nil {
		return s.fail("persist", err)
	}

	s.progress(ClusterProgress{Step: "done", Index: totalSteps, Total: totalSteps,
		Done: true, Message: fmt.Sprintf("Replica set %q is ready", name)})
	return nil
}

// Dissolve returns every member to standalone operation and restarts them.
func (s *ClusterService) Dissolve(name string) error {
	state := s.core.store.State()

	var record store.ReplicaSetRecord
	found := false
	for _, candidate := range state.ReplicaSets {
		if candidate.Name == name {
			record, found = candidate, true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown replica set %q", name)
	}

	binary := s.core.Mongod()
	if binary.Path == "" {
		return ErrNoMongod
	}
	ctx := s.core.context()

	for _, instanceID := range record.MemberIDs {
		instance, ok := s.core.store.Instance(instanceID)
		if !ok {
			continue
		}

		if s.core.supervisor.IsRunning(instanceID) {
			if err := s.core.supervisor.Stop(ctx, instanceID); err != nil {
				return err
			}
		}
		if err := disableReplication(instance); err != nil {
			return err
		}
		if err := s.core.supervisor.Start(ctx, instance, binary.Path); err != nil {
			return err
		}
	}

	return s.core.store.Update(func(state *store.State) error {
		var remaining []store.ReplicaSetRecord
		for _, candidate := range state.ReplicaSets {
			if candidate.Name != name {
				remaining = append(remaining, candidate)
			}
		}
		state.ReplicaSets = remaining

		for i := range state.Instances {
			for _, memberID := range record.MemberIDs {
				if state.Instances[i].ID == memberID {
					state.Instances[i].ReplicaSet = ""
				}
			}
		}
		return nil
	})
}

func (s *ClusterService) collect(instanceIDs []string) ([]store.InstanceRecord, error) {
	if len(instanceIDs) < 2 {
		return nil, errors.New("a replica set needs at least two instances")
	}

	records := make([]store.InstanceRecord, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		record, ok := s.core.store.Instance(id)
		if !ok {
			return nil, fmt.Errorf("unknown instance %q", id)
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *ClusterService) persistGroup(name string, records []store.InstanceRecord) error {
	memberIDs := make([]string, 0, len(records))
	for _, record := range records {
		memberIDs = append(memberIDs, record.ID)
	}

	return s.core.store.Update(func(state *store.State) error {
		replaced := false
		for i := range state.ReplicaSets {
			if state.ReplicaSets[i].Name == name {
				state.ReplicaSets[i].MemberIDs = memberIDs
				replaced = true
				break
			}
		}
		if !replaced {
			state.ReplicaSets = append(state.ReplicaSets, store.ReplicaSetRecord{
				Name:      name,
				MemberIDs: memberIDs,
				CreatedAt: time.Now(),
			})
		}

		for i := range state.Instances {
			for _, memberID := range memberIDs {
				if state.Instances[i].ID == memberID {
					state.Instances[i].ReplicaSet = name
				}
			}
		}
		return nil
	})
}

func (s *ClusterService) firstRunningPort(record store.ReplicaSetRecord, state store.State) (int, bool) {
	for _, memberID := range record.MemberIDs {
		if !s.core.supervisor.IsRunning(memberID) {
			continue
		}
		for _, instance := range state.Instances {
			if instance.ID == memberID {
				return instance.Port, true
			}
		}
	}
	return 0, false
}

func (s *ClusterService) progress(update ClusterProgress) {
	s.core.emit(EventClusterProgress, update)
}

func (s *ClusterService) fail(step string, err error) error {
	s.progress(ClusterProgress{Step: step, Error: err.Error()})
	return err
}

func enableReplication(record store.InstanceRecord, name string) error {
	config, err := loadConfig(record.ConfigPath)
	if err != nil {
		return err
	}
	if err := config.SetReplSetName(name); err != nil {
		return err
	}
	if err := config.SetOplogSizeMB(replicaset.DefaultOplogSizeMB); err != nil {
		return err
	}
	return writeConfigFile(record.ConfigPath, config)
}

func disableReplication(record store.InstanceRecord) error {
	config, err := loadConfig(record.ConfigPath)
	if err != nil {
		return err
	}
	config.ClearReplSetName()
	return writeConfigFile(record.ConfigPath, config)
}

func loadConfig(path string) (*mongoconf.Doc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration %q: %w", path, err)
	}
	return mongoconf.Parse(data)
}

// clearDataDir empties a member's data directory while keeping its configuration
// file, which lives in the same folder.
func clearDataDir(dir string, configPath string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read data directory %q: %w", dir, err)
	}

	for _, entry := range entries {
		path := dir + string(os.PathSeparator) + entry.Name()
		if sameDir(path, configPath) {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("clear data directory %q: %w", dir, err)
		}
	}
	return nil
}
