// Package supervisor owns the lifecycle of managed mongod processes.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mongoctl/internal/logstream"
	"mongoctl/internal/mongoclient"
	"mongoctl/internal/store"
	"mongoctl/internal/winsys"
)

// Errors returned by lifecycle operations.
var (
	ErrAlreadyRunning = errors.New("instance is already running")
	ErrNotRunning     = errors.New("instance is not running")
	ErrPortBusy       = errors.New("port is already in use")
)

const (
	readyTimeout      = 30 * time.Second
	readyPollInterval = 250 * time.Millisecond
	readyPingTimeout  = 500 * time.Millisecond
	shutdownGrace     = 20 * time.Second
	logBufferLines    = 2000
)

// State is the lifecycle state of a managed instance.
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateFailed   State = "failed"
)

// Status is a snapshot of one instance's lifecycle, shaped for the UI.
type Status struct {
	InstanceID string    `json:"instanceId"`
	State      State     `json:"state"`
	PID        int       `json:"pid"`
	Port       int       `json:"port"`
	StartedAt  time.Time `json:"startedAt"`
	Error      string    `json:"error"`

	// UncleanStop marks a process that had to be killed. The next start will run
	// WiredTiger recovery, which the UI warns about.
	UncleanStop bool `json:"uncleanStop"`
}

// Supervisor starts, stops and watches mongod processes.
type Supervisor struct {
	group    *winsys.ProcessGroup
	logDir   string
	onStatus func(Status)
	onLog    func(instanceID string, line logstream.Line)

	mu        sync.Mutex
	processes map[string]*process
}

// New creates a supervisor. Callbacks may be nil and are invoked without locks held.
func New(group *winsys.ProcessGroup, logDir string, onStatus func(Status), onLog func(string, logstream.Line)) *Supervisor {
	return &Supervisor{
		group:     group,
		logDir:    logDir,
		onStatus:  onStatus,
		onLog:     onLog,
		processes: map[string]*process{},
	}
}

// Start launches mongod for the given instance and waits until it accepts commands.
func (s *Supervisor) Start(ctx context.Context, record store.InstanceRecord, mongodPath string) error {
	s.mu.Lock()
	if existing, ok := s.processes[record.ID]; ok && existing.alive() {
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlreadyRunning, record.Name)
	}
	s.mu.Unlock()

	if err := os.MkdirAll(record.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory %q: %w", record.DataDir, err)
	}
	if !winsys.PortFree(record.Port) {
		return fmt.Errorf("%w: %d", ErrPortBusy, record.Port)
	}

	proc, err := s.spawn(record, mongodPath)
	if err != nil {
		s.emit(Status{InstanceID: record.ID, State: StateFailed, Port: record.Port, Error: err.Error()})
		return err
	}

	s.mu.Lock()
	s.processes[record.ID] = proc
	s.mu.Unlock()
	s.emit(proc.status())

	if err := s.waitReady(ctx, proc); err != nil {
		proc.setFailed(err)
		s.emit(proc.status())
		return err
	}

	proc.setState(StateRunning)
	s.emit(proc.status())
	return nil
}

// Stop asks mongod to shut down cleanly, escalating to a kill if it will not exit.
func (s *Supervisor) Stop(ctx context.Context, instanceID string) error {
	s.mu.Lock()
	proc, ok := s.processes[instanceID]
	s.mu.Unlock()

	if !ok || !proc.alive() {
		return fmt.Errorf("%w: %s", ErrNotRunning, instanceID)
	}

	proc.setState(StateStopping)
	s.emit(proc.status())

	// The shutdown command's own error is not a reliable signal: mongod drops the
	// connection while replying, so the driver reports a network failure on the
	// happy path. Process exit is the authoritative outcome.
	shutdownErr := requestShutdown(ctx, proc.port)

	select {
	case <-proc.exited:
		proc.setState(StateStopped)
		s.emit(proc.status())
		return nil
	case <-time.After(shutdownGrace):
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := proc.kill(); err != nil {
		return fmt.Errorf("force-kill mongod (clean shutdown failed: %v): %w", shutdownErr, err)
	}
	<-proc.exited

	proc.markUnclean()
	proc.setState(StateStopped)
	s.emit(proc.status())
	return nil
}

// Restart stops the instance if it is running, then starts it again.
func (s *Supervisor) Restart(ctx context.Context, record store.InstanceRecord, mongodPath string) error {
	if s.IsRunning(record.ID) {
		if err := s.Stop(ctx, record.ID); err != nil {
			return err
		}
	}
	return s.Start(ctx, record, mongodPath)
}

// StopAll shuts every running instance down in parallel, for application exit.
func (s *Supervisor) StopAll(ctx context.Context) {
	s.mu.Lock()
	ids := make([]string, 0, len(s.processes))
	for id, proc := range s.processes {
		if proc.alive() {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(instanceID string) {
			defer wg.Done()
			_ = s.Stop(ctx, instanceID)
		}(id)
	}
	wg.Wait()
}

// IsRunning reports whether the supervisor currently owns a live process.
func (s *Supervisor) IsRunning(instanceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	proc, ok := s.processes[instanceID]
	return ok && proc.alive()
}

// Status returns the last known lifecycle state of an instance.
func (s *Supervisor) Status(instanceID string) Status {
	s.mu.Lock()
	proc, ok := s.processes[instanceID]
	s.mu.Unlock()

	if !ok {
		return Status{InstanceID: instanceID, State: StateStopped}
	}
	return proc.status()
}

// Logs returns the retained log window for an instance.
func (s *Supervisor) Logs(instanceID string) []logstream.Line {
	s.mu.Lock()
	proc, ok := s.processes[instanceID]
	s.mu.Unlock()

	if !ok {
		return nil
	}
	return proc.stream.Snapshot()
}

func (s *Supervisor) spawn(record store.InstanceRecord, mongodPath string) (*process, error) {
	cmd := exec.Command(mongodPath, "--config", record.ConfigPath)
	cmd.SysProcAttr = winsys.HiddenProcAttr()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("capture mongod stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("capture mongod stderr: %w", err)
	}

	logPath := filepath.Join(s.logDir, record.ID+".log")
	stream, err := logstream.NewStream(logPath, logBufferLines, func(line logstream.Line) {
		if s.onLog != nil {
			s.onLog(record.ID, line)
		}
	})
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stream.Close()
		return nil, fmt.Errorf("start mongod: %w", err)
	}

	if err := s.group.Adopt(cmd.Process.Pid); err != nil {
		// Supervision still works; only crash-cleanup protection is reduced.
		stream.Append(logstream.Line{
			Timestamp: time.Now(),
			Severity:  logstream.SeverityWarning,
			Component: "MONGOCTL",
			Message:   fmt.Sprintf("could not place mongod in the process group: %v", err),
		})
	}

	proc := &process{
		id:        record.ID,
		port:      record.Port,
		cmd:       cmd,
		stream:    stream,
		exited:    make(chan struct{}),
		state:     StateStarting,
		startedAt: time.Now(),
	}

	var consumers sync.WaitGroup
	consumers.Add(2)
	go func() { defer consumers.Done(); stream.Consume(stdout) }()
	go func() { defer consumers.Done(); stream.Consume(stderr) }()

	go func() {
		consumers.Wait()
		waitErr := cmd.Wait()
		stream.Close()

		proc.mu.Lock()
		proc.waitErr = waitErr
		if proc.state == StateRunning || proc.state == StateStarting {
			proc.state = StateFailed
			if proc.err == nil {
				proc.err = fmt.Errorf("mongod exited unexpectedly: %s", describeExit(cmd, waitErr))
			}
		}
		proc.mu.Unlock()

		close(proc.exited)
		s.emit(proc.status())
	}()

	return proc, nil
}

func (s *Supervisor) waitReady(ctx context.Context, proc *process) error {
	ticker := time.NewTicker(readyPollInterval)
	defer ticker.Stop()
	deadline := time.NewTimer(readyTimeout)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-proc.exited:
			return fmt.Errorf("mongod exited during startup: %s", describeExit(proc.cmd, proc.waitError()))
		case <-deadline.C:
			return fmt.Errorf("mongod did not accept connections within %s", readyTimeout)
		case <-ticker.C:
			if err := mongoclient.Ping(ctx, proc.port, readyPingTimeout); err == nil {
				return nil
			}
		}
	}
}

func (s *Supervisor) emit(status Status) {
	if s.onStatus != nil {
		s.onStatus(status)
	}
}

func requestShutdown(ctx context.Context, port int) error {
	client, err := mongoclient.Connect(port, mongoclient.DefaultTimeout)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())

	// timeoutSecs is how long a replica set member waits for a secondary to catch
	// up before going down. Ten seconds is the usual default, but on a throwaway
	// local set it only makes every shutdown feel broken; a secondary that falls
	// behind catches up from the oplog on its next start.
	command := bson.D{
		{Key: "shutdown", Value: 1},
		{Key: "force", Value: true},
		{Key: "timeoutSecs", Value: 2},
	}
	return client.Database("admin").RunCommand(ctx, command).Err()
}

func describeExit(cmd *exec.Cmd, waitErr error) string {
	if cmd.ProcessState != nil {
		return fmt.Sprintf("exit code %d", cmd.ProcessState.ExitCode())
	}
	if waitErr != nil {
		return waitErr.Error()
	}
	return "unknown reason"
}

// process is one supervised mongod.
type process struct {
	id     string
	port   int
	cmd    *exec.Cmd
	stream *logstream.Stream
	exited chan struct{}

	mu        sync.Mutex
	state     State
	startedAt time.Time
	err       error
	unclean   bool
	waitErr   error
}

func (p *process) alive() bool {
	select {
	case <-p.exited:
		return false
	default:
		return true
	}
}

func (p *process) setState(state State) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = state
}

func (p *process) setFailed(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = StateFailed
	p.err = err
}

func (p *process) markUnclean() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unclean = true
}

func (p *process) waitError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

func (p *process) kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func (p *process) status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()

	status := Status{
		InstanceID:  p.id,
		State:       p.state,
		Port:        p.port,
		StartedAt:   p.startedAt,
		UncleanStop: p.unclean,
	}
	if p.cmd.Process != nil {
		status.PID = p.cmd.Process.Pid
	}
	if p.err != nil {
		status.Error = p.err.Error()
	}
	return status
}
