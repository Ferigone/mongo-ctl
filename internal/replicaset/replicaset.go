// Package replicaset performs the MongoDB-protocol half of replica set
// management: initiating a set and reporting its health.
//
// Enlisting a server in a replica set also requires replication.replSetName in
// its configuration and a restart, because mongod only reads that setting at
// startup. Orchestrating those restarts is the caller's job.
package replicaset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mongoctl/internal/mongoclient"
)

const (
	// DefaultOplogSizeMB keeps the oplog small; the default of 5% of free disk is
	// wasteful for throwaway local sets.
	DefaultOplogSizeMB = 128

	primaryPollInterval = 500 * time.Millisecond
)

// Member is one replica set member as reported by replSetGetStatus.
type Member struct {
	ID         int       `json:"id"`
	Host       string    `json:"host"`
	StateStr   string    `json:"stateStr"`
	Health     float64   `json:"health"`
	UptimeSecs int64     `json:"uptimeSecs"`
	OptimeDate time.Time `json:"optimeDate"`
	LagSeconds float64   `json:"lagSeconds"`
	IsPrimary  bool      `json:"isPrimary"`
	Self       bool      `json:"self"`
}

// Status is the health of a replica set.
type Status struct {
	Name    string   `json:"name"`
	Members []Member `json:"members"`
}

// Primary returns the current primary, if one has been elected.
func (s Status) Primary() (Member, bool) {
	for _, member := range s.Members {
		if member.IsPrimary {
			return member, true
		}
	}
	return Member{}, false
}

// Host formats a loopback member address.
func Host(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// Initiate creates a replica set from servers already running with the matching
// replSetName. Members are addressed by loopback; mixing loopback with real
// hostnames is rejected by mongod, so every local member must use 127.0.0.1.
func Initiate(ctx context.Context, name string, ports []int) error {
	if len(ports) == 0 {
		return fmt.Errorf("replica set %q needs at least one member", name)
	}

	members := make([]bson.M, 0, len(ports))
	for index, port := range ports {
		members = append(members, bson.M{"_id": index, "host": Host(port)})
	}

	client, err := mongoclient.Connect(ports[0], mongoclient.DefaultTimeout)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())

	config := bson.M{"_id": name, "members": members}
	command := bson.D{{Key: "replSetInitiate", Value: config}}

	if err := client.Database("admin").RunCommand(ctx, command).Err(); err != nil {
		return fmt.Errorf("initiate replica set %q: %w", name, err)
	}
	return nil
}

// GetStatus reads replSetGetStatus from any member.
func GetStatus(ctx context.Context, port int) (Status, error) {
	client, err := mongoclient.Connect(port, mongoclient.DefaultTimeout)
	if err != nil {
		return Status{}, err
	}
	defer client.Disconnect(context.Background())

	var raw statusResult
	command := bson.D{{Key: "replSetGetStatus", Value: 1}}
	if err := client.Database("admin").RunCommand(ctx, command).Decode(&raw); err != nil {
		return Status{}, fmt.Errorf("read replica set status on port %d: %w", port, err)
	}

	return raw.toStatus(), nil
}

// WaitForPrimary blocks until the set has elected a primary.
func WaitForPrimary(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(primaryPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			if lastErr != nil {
				return fmt.Errorf("no primary elected within %s: %w", timeout, lastErr)
			}
			return fmt.Errorf("no primary elected within %s", timeout)
		case <-ticker.C:
			status, err := GetStatus(ctx, port)
			if err != nil {
				lastErr = err
				continue
			}
			if _, ok := status.Primary(); ok {
				return nil
			}
		}
	}
}

// DataDirPopulated reports whether a data directory already holds a database.
// A member other than the seed must start empty, because existing data blocks
// the initial sync that populates it from the primary.
func DataDirPopulated(dir string) (bool, error) {
	for _, marker := range []string{"WiredTiger", "storage.bson"} {
		_, err := os.Stat(filepath.Join(dir, marker))
		if err == nil {
			return true, nil
		}
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("inspect data directory %q: %w", dir, err)
		}
	}
	return false, nil
}

// statusResult decodes the replSetGetStatus reply.
type statusResult struct {
	Set     string `bson:"set"`
	Members []struct {
		ID         int       `bson:"_id"`
		Name       string    `bson:"name"`
		Health     float64   `bson:"health"`
		State      int       `bson:"state"`
		StateStr   string    `bson:"stateStr"`
		Uptime     int64     `bson:"uptime"`
		OptimeDate time.Time `bson:"optimeDate"`
		Self       bool      `bson:"self"`
	} `bson:"members"`
}

// primaryState is the numeric member state mongod uses for PRIMARY.
const primaryState = 1

func (r statusResult) toStatus() Status {
	status := Status{Name: r.Set}

	var primaryOptime time.Time
	for _, member := range r.Members {
		if member.State == primaryState {
			primaryOptime = member.OptimeDate
			break
		}
	}

	for _, member := range r.Members {
		converted := Member{
			ID:         member.ID,
			Host:       member.Name,
			StateStr:   member.StateStr,
			Health:     member.Health,
			UptimeSecs: member.Uptime,
			OptimeDate: member.OptimeDate,
			IsPrimary:  member.State == primaryState,
			Self:       member.Self,
		}
		if !primaryOptime.IsZero() && !member.OptimeDate.IsZero() && member.State != primaryState {
			converted.LagSeconds = primaryOptime.Sub(member.OptimeDate).Seconds()
		}
		status.Members = append(status.Members, converted)
	}
	return status
}
