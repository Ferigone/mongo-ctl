// Package logstream captures mongod output, parses its structured log format
// and keeps a bounded window of recent lines for the UI.
package logstream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Severity is a mongod log level.
type Severity string

const (
	SeverityFatal   Severity = "fatal"
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityDebug   Severity = "debug"
)

// Line is one parsed log record.
type Line struct {
	Timestamp time.Time `json:"timestamp"`
	Severity  Severity  `json:"severity"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
	Raw       string    `json:"raw"`
}

// Buffer is a fixed-capacity ring of log lines.
type Buffer struct {
	mu    sync.RWMutex
	lines []Line
	next  int
	full  bool
}

// NewBuffer creates a ring holding at most capacity lines.
func NewBuffer(capacity int) *Buffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &Buffer{lines: make([]Line, capacity)}
}

// Append records a line, discarding the oldest once capacity is reached.
func (b *Buffer) Append(line Line) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines[b.next] = line
	b.next = (b.next + 1) % len(b.lines)
	if b.next == 0 {
		b.full = true
	}
}

// Snapshot returns the buffered lines in chronological order.
func (b *Buffer) Snapshot() []Line {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.full {
		return append([]Line(nil), b.lines[:b.next]...)
	}
	result := make([]Line, 0, len(b.lines))
	result = append(result, b.lines[b.next:]...)
	result = append(result, b.lines[:b.next]...)
	return result
}

// Stream consumes a mongod output pipe into a log file and an in-memory ring.
type Stream struct {
	buffer *Buffer
	onLine func(Line)

	mu   sync.Mutex
	file *os.File
}

// NewStream opens path for writing, truncating any log from a previous run.
func NewStream(path string, capacity int, onLine func(Line)) (*Stream, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create log file %q: %w", path, err)
	}
	return &Stream{buffer: NewBuffer(capacity), onLine: onLine, file: file}, nil
}

// Consume reads reader until EOF. It is meant to run in its own goroutine.
func (s *Stream) Consume(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}

		line := Parse(text)
		s.buffer.Append(line)
		s.write(text)
		if s.onLine != nil {
			s.onLine(line)
		}
	}
}

// Snapshot returns the retained log window.
func (s *Stream) Snapshot() []Line { return s.buffer.Snapshot() }

// Append records a line the supervisor produced itself, so operational warnings
// appear in the same timeline as mongod's own output.
func (s *Stream) Append(line Line) {
	s.buffer.Append(line)
	s.write(line.Message)
	if s.onLine != nil {
		s.onLine(line)
	}
}

// Close releases the log file.
func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

func (s *Stream) write(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		fmt.Fprintln(s.file, text)
	}
}

// logv2Entry is the JSON record mongod 4.4+ writes for every log line.
type logv2Entry struct {
	Timestamp struct {
		Date time.Time `json:"$date"`
	} `json:"t"`
	Severity  string `json:"s"`
	Component string `json:"c"`
	Message   string `json:"msg"`
}

// Parse turns a mongod output line into a Line. Text that is not structured JSON
// — startup banners and crash output — is kept verbatim at info severity.
func Parse(text string) Line {
	var entry logv2Entry
	if err := json.Unmarshal([]byte(text), &entry); err != nil || entry.Message == "" {
		return Line{Timestamp: time.Now(), Severity: SeverityInfo, Message: text, Raw: text}
	}

	timestamp := entry.Timestamp.Date
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	return Line{
		Timestamp: timestamp,
		Severity:  mapSeverity(entry.Severity),
		Component: entry.Component,
		Message:   entry.Message,
		Raw:       text,
	}
}

func mapSeverity(code string) Severity {
	switch code {
	case "F":
		return SeverityFatal
	case "E":
		return SeverityError
	case "W":
		return SeverityWarning
	case "D", "D1", "D2", "D3", "D4", "D5":
		return SeverityDebug
	default:
		return SeverityInfo
	}
}
