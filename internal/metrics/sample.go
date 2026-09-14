// Package metrics polls mongod for performance counters and keeps a rolling
// window of samples for charting.
package metrics

import (
	"sync"
	"time"
)

// Sample is one poll of serverStatus reduced to the values the UI charts.
//
// Rate fields are derived from cumulative counters and therefore need two polls;
// the first sample after a server starts carries zero rates and Gap set.
type Sample struct {
	Timestamp time.Time `json:"timestamp"`

	ConnectionsCurrent   int     `json:"connectionsCurrent"`
	ConnectionsActive    int     `json:"connectionsActive"`
	ConnectionsAvailable int     `json:"connectionsAvailable"`
	MemResidentMB        int     `json:"memResidentMb"`
	MemVirtualMB         int     `json:"memVirtualMb"`
	CacheUsedBytes       int64   `json:"cacheUsedBytes"`
	CacheMaxBytes        int64   `json:"cacheMaxBytes"`
	CacheFillPercent     float64 `json:"cacheFillPercent"`
	QueueReaders         int     `json:"queueReaders"`
	QueueWriters         int     `json:"queueWriters"`

	InsertsPerSec  float64 `json:"insertsPerSec"`
	QueriesPerSec  float64 `json:"queriesPerSec"`
	UpdatesPerSec  float64 `json:"updatesPerSec"`
	DeletesPerSec  float64 `json:"deletesPerSec"`
	CommandsPerSec float64 `json:"commandsPerSec"`
	GetMoresPerSec float64 `json:"getMoresPerSec"`
	BytesInPerSec  float64 `json:"bytesInPerSec"`
	BytesOutPerSec float64 `json:"bytesOutPerSec"`

	ReadLatencyMs    float64 `json:"readLatencyMs"`
	WriteLatencyMs   float64 `json:"writeLatencyMs"`
	CommandLatencyMs float64 `json:"commandLatencyMs"`

	// Gap marks a sample whose rates are not comparable to the previous one,
	// because the server restarted and its counters went back to zero. Charts
	// break the line here instead of drawing a meaningless spike.
	Gap bool `json:"gap"`
}

// Ring is a fixed-capacity buffer of samples in chronological order.
type Ring struct {
	mu      sync.RWMutex
	samples []Sample
	next    int
	full    bool
}

// NewRing creates a ring holding at most capacity samples.
func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = 1
	}
	return &Ring{samples: make([]Sample, capacity)}
}

// Append stores a sample, overwriting the oldest once capacity is reached.
func (r *Ring) Append(sample Sample) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.samples[r.next] = sample
	r.next = (r.next + 1) % len(r.samples)
	if r.next == 0 {
		r.full = true
	}
}

// Snapshot returns the retained samples, oldest first.
func (r *Ring) Snapshot() []Sample {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.full {
		return append([]Sample(nil), r.samples[:r.next]...)
	}
	result := make([]Sample, 0, len(r.samples))
	result = append(result, r.samples[r.next:]...)
	result = append(result, r.samples[:r.next]...)
	return result
}

// derive builds a Sample from the current serverStatus and the previous poll.
func derive(current *serverStatus, previous *serverStatus, elapsed time.Duration) Sample {
	sample := Sample{
		Timestamp:            time.Now(),
		ConnectionsCurrent:   current.Connections.Current,
		ConnectionsActive:    current.Connections.Active,
		ConnectionsAvailable: current.Connections.Available,
		MemResidentMB:        current.Mem.Resident,
		MemVirtualMB:         current.Mem.Virtual,
		QueueReaders:         current.GlobalLock.CurrentQueue.Readers,
		QueueWriters:         current.GlobalLock.CurrentQueue.Writers,
		CacheUsedBytes:       current.cacheUsed(),
		CacheMaxBytes:        current.cacheMax(),
	}
	if sample.CacheMaxBytes > 0 {
		sample.CacheFillPercent = float64(sample.CacheUsedBytes) / float64(sample.CacheMaxBytes) * 100
	}

	seconds := elapsed.Seconds()
	if previous == nil || seconds <= 0 || current.Uptime < previous.Uptime {
		sample.Gap = true
		return sample
	}

	sample.InsertsPerSec = perSecond(current.Opcounters.Insert, previous.Opcounters.Insert, seconds)
	sample.QueriesPerSec = perSecond(current.Opcounters.Query, previous.Opcounters.Query, seconds)
	sample.UpdatesPerSec = perSecond(current.Opcounters.Update, previous.Opcounters.Update, seconds)
	sample.DeletesPerSec = perSecond(current.Opcounters.Delete, previous.Opcounters.Delete, seconds)
	sample.CommandsPerSec = perSecond(current.Opcounters.Command, previous.Opcounters.Command, seconds)
	sample.GetMoresPerSec = perSecond(current.Opcounters.GetMore, previous.Opcounters.GetMore, seconds)
	sample.BytesInPerSec = perSecond(current.Network.BytesIn, previous.Network.BytesIn, seconds)
	sample.BytesOutPerSec = perSecond(current.Network.BytesOut, previous.Network.BytesOut, seconds)

	sample.ReadLatencyMs = averageLatencyMs(current.OpLatencies.Reads, previous.OpLatencies.Reads)
	sample.WriteLatencyMs = averageLatencyMs(current.OpLatencies.Writes, previous.OpLatencies.Writes)
	sample.CommandLatencyMs = averageLatencyMs(current.OpLatencies.Commands, previous.OpLatencies.Commands)

	return sample
}

func perSecond(current, previous int64, seconds float64) float64 {
	delta := current - previous
	if delta < 0 {
		return 0
	}
	return float64(delta) / seconds
}

// averageLatencyMs converts the latency/ops counter pair into the mean latency of
// the operations that happened between two polls. mongod reports microseconds.
func averageLatencyMs(current, previous latencyStat) float64 {
	deltaOps := current.Ops - previous.Ops
	deltaLatency := current.Latency - previous.Latency
	if deltaOps <= 0 || deltaLatency < 0 {
		return 0
	}
	return float64(deltaLatency) / float64(deltaOps) / 1000
}
