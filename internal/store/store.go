// Package store persists the instance registry and application settings.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const stateFileName = "state.json"

// DefaultBasePort starts above 27017, which the stock MongoDB Windows service owns.
const DefaultBasePort = 27018

// Settings holds user-level preferences.
type Settings struct {
	DataRoot   string `json:"dataRoot"`
	MongodPath string `json:"mongodPath"`
	BasePort   int    `json:"basePort"`
}

// InstanceRecord is the persisted metadata of a managed mongod instance. The
// authoritative server configuration lives in the instance's mongod.conf, not here.
type InstanceRecord struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	DataDir    string    `json:"dataDir"`
	ConfigPath string    `json:"configPath"`
	Port       int       `json:"port"`
	ReplicaSet string    `json:"replicaSet"`
	CreatedAt  time.Time `json:"createdAt"`

	// LastPID and LastStart let a restarted GUI recognise processes it left behind.
	LastPID   int       `json:"lastPid"`
	LastStart time.Time `json:"lastStart"`
}

// ReplicaSetRecord groups instances that were initiated together.
type ReplicaSetRecord struct {
	Name      string    `json:"name"`
	MemberIDs []string  `json:"memberIds"`
	CreatedAt time.Time `json:"createdAt"`
}

// State is the full persisted document.
type State struct {
	Settings    Settings           `json:"settings"`
	Instances   []InstanceRecord   `json:"instances"`
	ReplicaSets []ReplicaSetRecord `json:"replicaSets"`
}

// Store reads and writes State, serialising access across goroutines.
type Store struct {
	path  string
	mu    sync.RWMutex
	state State
}

// Open loads the state file from dir, creating it with defaults when absent.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create state directory %q: %w", dir, err)
	}

	store := &Store{path: filepath.Join(dir, stateFileName)}

	data, err := os.ReadFile(store.path)
	switch {
	case os.IsNotExist(err):
		store.state = State{Settings: defaultSettings()}
		if err := store.persist(); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, fmt.Errorf("read state file: %w", err)
	default:
		if err := json.Unmarshal(data, &store.state); err != nil {
			return nil, fmt.Errorf("parse state file %q: %w", store.path, err)
		}
		store.state.Settings = withDefaults(store.state.Settings)
	}

	return store, nil
}

// State returns a deep copy safe to hand to other goroutines and the UI.
func (s *Store) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneState(s.state)
}

// Update applies fn to the state and writes the result. The state is left
// untouched if fn returns an error, so a rejected change cannot corrupt the file.
func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	draft := cloneState(s.state)
	if err := fn(&draft); err != nil {
		return err
	}

	previous := s.state
	s.state = draft
	if err := s.persist(); err != nil {
		s.state = previous
		return err
	}
	return nil
}

// Instance returns the record with the given ID.
func (s *Store) Instance(id string) (InstanceRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, record := range s.state.Instances {
		if record.ID == id {
			return record, true
		}
	}
	return InstanceRecord{}, false
}

// persist writes the state through a temporary file so a crash mid-write cannot
// leave a truncated registry behind. Callers must hold the write lock.
func (s *Store) persist() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	dir := filepath.Dir(s.path)
	temp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tempName := temp.Name()

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempName)
		return fmt.Errorf("write temporary state file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		os.Remove(tempName)
		return fmt.Errorf("flush temporary state file: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return fmt.Errorf("close temporary state file: %w", err)
	}

	// Windows refuses to rename onto an existing file.
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		os.Remove(tempName)
		return fmt.Errorf("replace state file: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		os.Remove(tempName)
		return fmt.Errorf("commit state file: %w", err)
	}
	return nil
}

func defaultSettings() Settings {
	return withDefaults(Settings{})
}

func withDefaults(settings Settings) Settings {
	if settings.BasePort == 0 {
		settings.BasePort = DefaultBasePort
	}
	if settings.DataRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		settings.DataRoot = filepath.Join(home, "MongoData")
	}
	return settings
}

func cloneState(state State) State {
	clone := State{Settings: state.Settings}

	if state.Instances != nil {
		clone.Instances = append([]InstanceRecord(nil), state.Instances...)
	}
	for i := range state.ReplicaSets {
		record := state.ReplicaSets[i]
		record.MemberIDs = append([]string(nil), record.MemberIDs...)
		clone.ReplicaSets = append(clone.ReplicaSets, record)
	}
	return clone
}
