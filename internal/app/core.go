// Package app wires the domain packages together and exposes them to the UI.
// It is the only package that imports the Wails runtime, so the rest of the
// backend stays framework-agnostic.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mongoctl/internal/logstream"
	"mongoctl/internal/metrics"
	"mongoctl/internal/mongobin"
	"mongoctl/internal/store"
	"mongoctl/internal/supervisor"
	"mongoctl/internal/winsys"
)

// Event names pushed to the frontend.
const (
	EventInstanceStatus  = "instance:status"
	EventInstanceLog     = "instance:log"
	EventInstanceMetrics = "instance:metrics"
	EventShutdown        = "app:shutdown"
)

const shutdownTimeout = 45 * time.Second

// Core holds the shared dependencies of every bound service.
type Core struct {
	store      *store.Store
	supervisor *supervisor.Supervisor
	metrics    *metrics.Manager
	group      *winsys.ProcessGroup

	shuttingDown atomic.Bool

	mu     sync.RWMutex
	ctx    context.Context
	mongod mongobin.Binary
}

// NewCore prepares the application state directories and the process group.
func NewCore() (*Core, error) {
	stateDir, logDir, err := applicationDirs()
	if err != nil {
		return nil, err
	}

	stateStore, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}

	group, err := winsys.NewProcessGroup()
	if err != nil {
		return nil, err
	}

	core := &Core{store: stateStore, group: group}

	core.supervisor = supervisor.New(group, logDir, core.handleStatus, core.handleLog)
	core.metrics = metrics.NewManager(metrics.DefaultInterval, metrics.DefaultCapacity, core.handleSample)

	return core, nil
}

// Startup receives the Wails context and resolves the mongod binary to use.
func (c *Core) Startup(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	c.mu.Unlock()

	c.resolveMongod(ctx)
}

// Shutdown stops every managed server before the window closes.
//
// Wails runs this on the UI thread, so the work must not happen inline: stopping
// a server emits status events, and delivering one needs the very thread we would
// be blocking, which deadlocks the window into "not responding". Instead the
// first close is vetoed, cleanup runs on its own goroutine, and Quit closes the
// app once the servers are actually down. The second call — from that Quit —
// sees the flag and lets the window go.
func (c *Core) Shutdown(ctx context.Context) bool {
	if c.shuttingDown.Swap(true) {
		return false
	}

	c.emit(EventShutdown, true)

	go func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		c.supervisor.StopAll(stopCtx)
		c.metrics.StopAll()
		c.group.Close()

		wailsruntime.Quit(ctx)
	}()

	return true
}

// Mongod returns the mongod binary the app will launch.
func (c *Core) Mongod() mongobin.Binary {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mongod
}

// resolveMongod picks the configured binary, falling back to the newest installed.
func (c *Core) resolveMongod(ctx context.Context) {
	settings := c.store.State().Settings

	if settings.MongodPath != "" {
		if binary, err := mongobin.Probe(ctx, settings.MongodPath); err == nil {
			c.setMongod(binary)
			return
		}
	}

	binaries, err := mongobin.Discover(ctx)
	if err != nil || len(binaries) == 0 {
		return
	}
	c.setMongod(binaries[0])
}

func (c *Core) setMongod(binary mongobin.Binary) {
	c.mu.Lock()
	c.mongod = binary
	c.mu.Unlock()
}

func (c *Core) context() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *Core) emit(event string, data any) {
	c.mu.RLock()
	ctx := c.ctx
	c.mu.RUnlock()

	if ctx == nil {
		return
	}
	wailsruntime.EventsEmit(ctx, event, data)
}

// handleStatus mirrors lifecycle changes to the UI and keeps metric sampling
// aligned with which servers are actually up.
func (c *Core) handleStatus(status supervisor.Status) {
	switch status.State {
	case supervisor.StateRunning:
		c.metrics.Start(status.InstanceID, status.Port)
	case supervisor.StateStopped, supervisor.StateFailed:
		go c.metrics.Stop(status.InstanceID)
	}
	c.emit(EventInstanceStatus, status)
}

func (c *Core) handleLog(instanceID string, line logstream.Line) {
	c.emit(EventInstanceLog, map[string]any{"instanceId": instanceID, "line": line})
}

func (c *Core) handleSample(instanceID string, sample metrics.Sample) {
	c.emit(EventInstanceMetrics, map[string]any{"instanceId": instanceID, "sample": sample})
}

// applicationDirs returns the state and log directories, creating them if needed.
func applicationDirs() (stateDir string, logDir string, err error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("locate application data directory: %w", err)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", "", fmt.Errorf("locate application cache directory: %w", err)
	}

	stateDir = filepath.Join(appData, "MongoCtl")
	logDir = filepath.Join(cacheDir, "MongoCtl", "logs")

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create log directory: %w", err)
	}
	return stateDir, logDir, nil
}
