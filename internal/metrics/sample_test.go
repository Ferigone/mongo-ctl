package metrics

import (
	"testing"
	"time"
)

func statusFixture() *serverStatus {
	return &serverStatus{
		Uptime: 100,
		Connections: connectionStats{
			Current:   12,
			Active:    3,
			Available: 800,
		},
		Mem:        memStats{Resident: 256, Virtual: 2048},
		Opcounters: opcounters{Insert: 1000, Query: 2000, Update: 300, Delete: 50, Command: 5000},
		Network:    networkStats{BytesIn: 10_000, BytesOut: 20_000},
		WiredTiger: &wiredTigerStats{
			Cache: map[string]any{
				cacheUsedKey: int64(50_000_000),
				cacheMaxKey:  int64(100_000_000),
			},
		},
	}
}

func TestDeriveWithoutPreviousMarksGap(t *testing.T) {
	sample := derive(statusFixture(), nil, time.Second)

	if !sample.Gap {
		t.Error("first sample must be a gap: rates need two polls")
	}
	if sample.InsertsPerSec != 0 {
		t.Errorf("InsertsPerSec = %v, want 0 without a baseline", sample.InsertsPerSec)
	}
	if sample.ConnectionsCurrent != 12 {
		t.Errorf("ConnectionsCurrent = %d, want 12; gauges are valid on the first sample", sample.ConnectionsCurrent)
	}
	if sample.CacheFillPercent != 50 {
		t.Errorf("CacheFillPercent = %v, want 50", sample.CacheFillPercent)
	}
}

func TestDeriveComputesRatesOverElapsedTime(t *testing.T) {
	previous := statusFixture()
	current := statusFixture()
	current.Uptime = 102
	current.Opcounters.Insert = 1200
	current.Opcounters.Query = 2400
	current.Network.BytesIn = 30_000

	sample := derive(current, previous, 2*time.Second)

	if sample.Gap {
		t.Fatal("a normal interval must not be reported as a gap")
	}
	if sample.InsertsPerSec != 100 {
		t.Errorf("InsertsPerSec = %v, want 100 (200 inserts over 2s)", sample.InsertsPerSec)
	}
	if sample.QueriesPerSec != 200 {
		t.Errorf("QueriesPerSec = %v, want 200", sample.QueriesPerSec)
	}
	if sample.BytesInPerSec != 10_000 {
		t.Errorf("BytesInPerSec = %v, want 10000", sample.BytesInPerSec)
	}
}

// A restarted server resets its counters. Reporting the drop as a negative rate
// would draw a spike that never happened.
func TestDeriveDetectsServerRestart(t *testing.T) {
	previous := statusFixture()
	current := statusFixture()
	current.Uptime = 3
	current.Opcounters.Insert = 5

	sample := derive(current, previous, time.Second)

	if !sample.Gap {
		t.Error("a shrinking uptime must be reported as a gap")
	}
	if sample.InsertsPerSec != 0 {
		t.Errorf("InsertsPerSec = %v, want 0 across a restart", sample.InsertsPerSec)
	}
}

func TestDeriveGuardsAgainstCounterGoingBackwards(t *testing.T) {
	previous := statusFixture()
	current := statusFixture()
	current.Uptime = 101
	current.Opcounters.Insert = previous.Opcounters.Insert - 10

	sample := derive(current, previous, time.Second)

	if sample.InsertsPerSec < 0 {
		t.Errorf("InsertsPerSec = %v, must never be negative", sample.InsertsPerSec)
	}
}

func TestAverageLatencyConvertsMicrosecondsToMilliseconds(t *testing.T) {
	previous := latencyStat{Latency: 1_000_000, Ops: 100}
	current := latencyStat{Latency: 1_400_000, Ops: 300}

	// 400000 microseconds over 200 operations = 2000 us = 2 ms each.
	if got := averageLatencyMs(current, previous); got != 2 {
		t.Errorf("averageLatencyMs = %v, want 2", got)
	}
}

func TestAverageLatencyWithoutOperations(t *testing.T) {
	stat := latencyStat{Latency: 500, Ops: 10}

	if got := averageLatencyMs(stat, stat); got != 0 {
		t.Errorf("averageLatencyMs = %v, want 0 when no operations happened", got)
	}
}

func TestMissingWiredTigerSectionIsTolerated(t *testing.T) {
	status := statusFixture()
	status.WiredTiger = nil

	sample := derive(status, nil, time.Second)

	if sample.CacheUsedBytes != 0 || sample.CacheFillPercent != 0 {
		t.Error("a server without WiredTiger must report zero cache usage, not fail")
	}
}

func TestRingKeepsMostRecentSamples(t *testing.T) {
	ring := NewRing(3)
	for i := 1; i <= 5; i++ {
		ring.Append(Sample{ConnectionsCurrent: i})
	}

	snapshot := ring.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("len(snapshot) = %d, want 3", len(snapshot))
	}
	for index, want := range []int{3, 4, 5} {
		if snapshot[index].ConnectionsCurrent != want {
			t.Errorf("snapshot[%d] = %d, want %d (order must be chronological)",
				index, snapshot[index].ConnectionsCurrent, want)
		}
	}
}

func TestRingBeforeWrapping(t *testing.T) {
	ring := NewRing(5)
	ring.Append(Sample{ConnectionsCurrent: 1})
	ring.Append(Sample{ConnectionsCurrent: 2})

	snapshot := ring.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("len(snapshot) = %d, want 2", len(snapshot))
	}
	if snapshot[0].ConnectionsCurrent != 1 || snapshot[1].ConnectionsCurrent != 2 {
		t.Error("partial ring must return samples in insertion order")
	}
}
