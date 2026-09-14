package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"mongoctl/internal/logstream"
	"mongoctl/internal/metrics"
	"mongoctl/internal/mongoconf"
	"mongoctl/internal/store"
	"mongoctl/internal/supervisor"
	"mongoctl/internal/winsys"
)

const configFileName = "mongod.conf"

// ErrNoMongod reports that no usable mongod executable was found.
var ErrNoMongod = errors.New("no mongod executable found; set its path in settings")

// InstanceService exposes instance management to the UI.
type InstanceService struct {
	core *Core
}

// NewInstanceService binds instance operations to the application core.
func NewInstanceService(core *Core) *InstanceService {
	return &InstanceService{core: core}
}

// InstanceView is an instance plus its live lifecycle state, flattened for the UI.
type InstanceView struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Port       int       `json:"port"`
	DataDir    string    `json:"dataDir"`
	ConfigPath string    `json:"configPath"`
	ReplicaSet string    `json:"replicaSet"`
	CreatedAt  time.Time `json:"createdAt"`

	State       supervisor.State `json:"state"`
	PID         int              `json:"pid"`
	StartedAt   time.Time        `json:"startedAt"`
	Error       string           `json:"error"`
	UncleanStop bool             `json:"uncleanStop"`
}

// CreateInstanceRequest describes a new instance. Port and DataDir are optional;
// zero and empty mean "choose sensible defaults".
type CreateInstanceRequest struct {
	Name    string `json:"name"`
	Port    int    `json:"port"`
	DataDir string `json:"dataDir"`
}

// List returns every managed instance with its current state.
func (s *InstanceService) List() []InstanceView {
	records := s.core.store.State().Instances

	views := make([]InstanceView, 0, len(records))
	for _, record := range records {
		views = append(views, s.view(record))
	}
	return views
}

// Create registers a new instance and writes its configuration file.
func (s *InstanceService) Create(request CreateInstanceRequest) (InstanceView, error) {
	name := strings.TrimSpace(request.Name)
	if err := validateName(name); err != nil {
		return InstanceView{}, err
	}

	state := s.core.store.State()
	for _, existing := range state.Instances {
		if strings.EqualFold(existing.Name, name) {
			return InstanceView{}, fmt.Errorf("an instance named %q already exists", name)
		}
	}

	port := request.Port
	if port == 0 {
		allocated, err := s.SuggestPort()
		if err != nil {
			return InstanceView{}, err
		}
		port = allocated
	} else if !winsys.PortFree(port) {
		return InstanceView{}, fmt.Errorf("port %d is already in use", port)
	}

	dataDir := strings.TrimSpace(request.DataDir)
	if dataDir == "" {
		dataDir = filepath.Join(state.Settings.DataRoot, slug(name))
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return InstanceView{}, fmt.Errorf("create data directory %q: %w", dataDir, err)
	}

	record := store.InstanceRecord{
		ID:         uuid.NewString(),
		Name:       name,
		Port:       port,
		DataDir:    dataDir,
		ConfigPath: filepath.Join(dataDir, configFileName),
		CreatedAt:  time.Now(),
	}

	config := mongoconf.ForInstance(dataDir, port)
	if err := writeConfigFile(record.ConfigPath, config); err != nil {
		return InstanceView{}, err
	}

	if err := s.core.store.Update(func(state *store.State) error {
		state.Instances = append(state.Instances, record)
		return nil
	}); err != nil {
		return InstanceView{}, err
	}

	return s.view(record), nil
}

// Remove stops the instance and forgets it. Data files are deleted only when
// deleteData is set, which the UI confirms separately.
func (s *InstanceService) Remove(id string, deleteData bool) error {
	record, ok := s.core.store.Instance(id)
	if !ok {
		return fmt.Errorf("unknown instance %q", id)
	}

	if s.core.supervisor.IsRunning(id) {
		if err := s.core.supervisor.Stop(s.core.context(), id); err != nil {
			return err
		}
	}
	s.core.metrics.Stop(id)

	if err := s.core.store.Update(func(state *store.State) error {
		state.Instances = removeRecord(state.Instances, id)
		state.ReplicaSets = detachMember(state.ReplicaSets, id)
		return nil
	}); err != nil {
		return err
	}

	if deleteData {
		if err := os.RemoveAll(record.DataDir); err != nil {
			return fmt.Errorf("delete data directory %q: %w", record.DataDir, err)
		}
	}
	return nil
}

// Start launches the instance and waits until it accepts connections.
func (s *InstanceService) Start(id string) error {
	record, binary, err := s.resolve(id)
	if err != nil {
		return err
	}
	return s.core.supervisor.Start(s.core.context(), record, binary)
}

// Stop shuts the instance down.
func (s *InstanceService) Stop(id string) error {
	return s.core.supervisor.Stop(s.core.context(), id)
}

// Restart stops the instance if needed and starts it again.
func (s *InstanceService) Restart(id string) error {
	record, binary, err := s.resolve(id)
	if err != nil {
		return err
	}
	return s.core.supervisor.Restart(s.core.context(), record, binary)
}

// Logs returns the retained log window for an instance.
func (s *InstanceService) Logs(id string) []logstream.Line {
	lines := s.core.supervisor.Logs(id)
	if lines == nil {
		return []logstream.Line{}
	}
	return lines
}

// MetricsHistory returns the retained metric samples, for chart hydration.
func (s *InstanceService) MetricsHistory(id string) []metrics.Sample {
	samples := s.core.metrics.History(id)
	if samples == nil {
		return []metrics.Sample{}
	}
	return samples
}

// Databases lists the databases on a running instance.
func (s *InstanceService) Databases(id string) ([]metrics.DatabaseInfo, error) {
	record, ok := s.core.store.Instance(id)
	if !ok {
		return nil, fmt.Errorf("unknown instance %q", id)
	}
	return metrics.Databases(s.core.context(), record.Port)
}

// ReadConfig returns the raw mongod.conf of an instance.
func (s *InstanceService) ReadConfig(id string) (string, error) {
	record, ok := s.core.store.Instance(id)
	if !ok {
		return "", fmt.Errorf("unknown instance %q", id)
	}

	data, err := os.ReadFile(record.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("read configuration %q: %w", record.ConfigPath, err)
	}
	return string(data), nil
}

// WriteConfig validates and saves an edited mongod.conf, then syncs the registry
// so a port or data directory changed by hand does not desynchronise the app.
func (s *InstanceService) WriteConfig(id string, content string) error {
	record, ok := s.core.store.Instance(id)
	if !ok {
		return fmt.Errorf("unknown instance %q", id)
	}

	config, err := mongoconf.Parse([]byte(content))
	if err != nil {
		return err
	}
	if port := config.Port(); port == 0 {
		return errors.New("configuration must set net.port")
	}
	if config.DBPath() == "" {
		return errors.New("configuration must set storage.dbPath")
	}

	if err := writeConfigFile(record.ConfigPath, config); err != nil {
		return err
	}

	return s.core.store.Update(func(state *store.State) error {
		for i := range state.Instances {
			if state.Instances[i].ID != id {
				continue
			}
			state.Instances[i].Port = config.Port()
			state.Instances[i].DataDir = config.DBPath()
			state.Instances[i].ReplicaSet = config.ReplSetName()
			return nil
		}
		return fmt.Errorf("unknown instance %q", id)
	})
}

// MoveDataDir relocates an instance's data files. The instance must be stopped.
func (s *InstanceService) MoveDataDir(id string, target string) error {
	record, ok := s.core.store.Instance(id)
	if !ok {
		return fmt.Errorf("unknown instance %q", id)
	}
	if s.core.supervisor.IsRunning(id) {
		return errors.New("stop the instance before moving its data directory")
	}

	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("target directory is required")
	}
	if sameDir(target, record.DataDir) {
		return nil
	}

	if err := moveDirectory(record.DataDir, target); err != nil {
		return err
	}

	data, err := os.ReadFile(record.ConfigPath)
	if err != nil {
		return fmt.Errorf("read configuration after move: %w", err)
	}
	config, err := mongoconf.Parse(data)
	if err != nil {
		return err
	}
	if err := config.SetDBPath(target); err != nil {
		return err
	}

	newConfigPath := filepath.Join(target, configFileName)
	if err := writeConfigFile(newConfigPath, config); err != nil {
		return err
	}

	return s.core.store.Update(func(state *store.State) error {
		for i := range state.Instances {
			if state.Instances[i].ID != id {
				continue
			}
			state.Instances[i].DataDir = target
			state.Instances[i].ConfigPath = newConfigPath
			return nil
		}
		return fmt.Errorf("unknown instance %q", id)
	})
}

// SuggestPort returns the next free port at or above the configured base.
func (s *InstanceService) SuggestPort() (int, error) {
	state := s.core.store.State()

	reserved := map[int]bool{}
	for _, record := range state.Instances {
		reserved[record.Port] = true
	}

	ctx, cancel := context.WithTimeout(s.core.context(), 10*time.Second)
	defer cancel()

	excluded, err := winsys.ExcludedPortRanges(ctx)
	if err != nil {
		// Reservation data is advisory; a bind failure would surface the conflict.
		excluded = nil
	}
	return winsys.FindFreePort(state.Settings.BasePort, reserved, excluded)
}

func (s *InstanceService) resolve(id string) (store.InstanceRecord, string, error) {
	record, ok := s.core.store.Instance(id)
	if !ok {
		return store.InstanceRecord{}, "", fmt.Errorf("unknown instance %q", id)
	}

	binary := s.core.Mongod()
	if binary.Path == "" {
		return store.InstanceRecord{}, "", ErrNoMongod
	}
	return record, binary.Path, nil
}

func (s *InstanceService) view(record store.InstanceRecord) InstanceView {
	status := s.core.supervisor.Status(record.ID)

	return InstanceView{
		ID:          record.ID,
		Name:        record.Name,
		Port:        record.Port,
		DataDir:     record.DataDir,
		ConfigPath:  record.ConfigPath,
		ReplicaSet:  record.ReplicaSet,
		CreatedAt:   record.CreatedAt,
		State:       status.State,
		PID:         status.PID,
		StartedAt:   status.StartedAt,
		Error:       status.Error,
		UncleanStop: status.UncleanStop,
	}
}

func writeConfigFile(path string, config *mongoconf.Doc) error {
	data, err := config.Bytes()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write configuration %q: %w", path, err)
	}
	return nil
}

var invalidNameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

func validateName(name string) error {
	if name == "" {
		return errors.New("instance name is required")
	}
	if invalidNameChars.MatchString(name) {
		return errors.New(`instance name cannot contain < > : " / \ | ? *`)
	}
	return nil
}

var slugSeparators = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func slug(name string) string {
	cleaned := slugSeparators.ReplaceAllString(name, "-")
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		return "instance"
	}
	return strings.ToLower(cleaned)
}

func removeRecord(records []store.InstanceRecord, id string) []store.InstanceRecord {
	result := records[:0]
	for _, record := range records {
		if record.ID != id {
			result = append(result, record)
		}
	}
	return result
}

func detachMember(sets []store.ReplicaSetRecord, instanceID string) []store.ReplicaSetRecord {
	var result []store.ReplicaSetRecord
	for _, set := range sets {
		members := make([]string, 0, len(set.MemberIDs))
		for _, member := range set.MemberIDs {
			if member != instanceID {
				members = append(members, member)
			}
		}
		if len(members) == 0 {
			continue
		}
		set.MemberIDs = members
		result = append(result, set)
	}
	return result
}

func sameDir(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
