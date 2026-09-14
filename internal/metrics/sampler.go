package metrics

import (
	"context"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mongoctl/internal/mongoclient"
)

// DefaultInterval is the poll cadence. One second is responsive enough to watch
// a load test without the polling itself showing up in the numbers.
const DefaultInterval = time.Second

// DefaultCapacity retains fifteen minutes of history at DefaultInterval.
const DefaultCapacity = 900

// Manager runs one sampling goroutine per running instance.
type Manager struct {
	interval time.Duration
	capacity int
	onSample func(instanceID string, sample Sample)

	mu       sync.Mutex
	samplers map[string]*sampler
}

type sampler struct {
	ring   *Ring
	cancel context.CancelFunc
	done   chan struct{}
}

// NewManager creates a metrics manager. onSample may be nil.
func NewManager(interval time.Duration, capacity int, onSample func(string, Sample)) *Manager {
	if interval <= 0 {
		interval = DefaultInterval
	}
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Manager{
		interval: interval,
		capacity: capacity,
		onSample: onSample,
		samplers: map[string]*sampler{},
	}
}

// Start begins sampling an instance. Sampling an instance that is already
// sampled is a no-op, so callers need not track whether they started it.
func (m *Manager) Start(instanceID string, port int) {
	m.mu.Lock()
	if _, exists := m.samplers[instanceID]; exists {
		m.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &sampler{
		ring:   NewRing(m.capacity),
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.samplers[instanceID] = entry
	m.mu.Unlock()

	go m.run(ctx, instanceID, port, entry)
}

// Stop ends sampling and discards the instance's history.
func (m *Manager) Stop(instanceID string) {
	m.mu.Lock()
	entry, ok := m.samplers[instanceID]
	delete(m.samplers, instanceID)
	m.mu.Unlock()

	if !ok {
		return
	}
	entry.cancel()
	<-entry.done
}

// StopAll ends every sampler, for application shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	entries := make([]*sampler, 0, len(m.samplers))
	for _, entry := range m.samplers {
		entries = append(entries, entry)
	}
	m.samplers = map[string]*sampler{}
	m.mu.Unlock()

	for _, entry := range entries {
		entry.cancel()
		<-entry.done
	}
}

// History returns the retained samples for an instance, oldest first.
func (m *Manager) History(instanceID string) []Sample {
	m.mu.Lock()
	entry, ok := m.samplers[instanceID]
	m.mu.Unlock()

	if !ok {
		return nil
	}
	return entry.ring.Snapshot()
}

func (m *Manager) run(ctx context.Context, instanceID string, port int, entry *sampler) {
	defer close(entry.done)

	client, err := mongoclient.Connect(port, mongoclient.DefaultTimeout)
	if err != nil {
		return
	}
	defer client.Disconnect(context.Background())

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	var (
		previous   *serverStatus
		previousAt time.Time
	)

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			current, err := poll(ctx, client)
			if err != nil {
				// The server is unreachable or restarting. Dropping the baseline
				// makes the next successful poll a gap rather than a false spike.
				previous = nil
				continue
			}

			sample := derive(current, previous, now.Sub(previousAt))
			entry.ring.Append(sample)
			if m.onSample != nil {
				m.onSample(instanceID, sample)
			}

			previous, previousAt = current, now
		}
	}
}

func poll(ctx context.Context, client *mongo.Client) (*serverStatus, error) {
	var status serverStatus
	command := bson.D{{Key: "serverStatus", Value: 1}}
	if err := client.Database("admin").RunCommand(ctx, command).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}
