// Package mongobin locates mongod executables installed on the machine and
// reports their versions.
package mongobin

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mongoctl/internal/winsys"
)

// Minimum server version the MongoDB Go driver v2 supports.
const (
	minimumMajor = 4
	minimumMinor = 4
)

const probeTimeout = 10 * time.Second

// Binary is a discovered mongod executable.
type Binary struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Major   int    `json:"major"`
	Minor   int    `json:"minor"`
	Patch   int    `json:"patch"`
}

// Supported reports whether the driver can talk to this server version.
func (b Binary) Supported() bool {
	if b.Major != minimumMajor {
		return b.Major > minimumMajor
	}
	return b.Minor >= minimumMinor
}

// searchGlobs are the standard MongoDB install locations on Windows.
var searchGlobs = []string{
	`C:\Program Files\MongoDB\Server\*\bin\mongod.exe`,
	`C:\Program Files (x86)\MongoDB\Server\*\bin\mongod.exe`,
}

var versionPattern = regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)`)

// Discover finds every installed mongod, newest version first. Executables that
// fail to report a version are skipped rather than failing the whole scan.
func Discover(ctx context.Context) ([]Binary, error) {
	candidates := map[string]struct{}{}

	for _, glob := range searchGlobs {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, fmt.Errorf("scan %q: %w", glob, err)
		}
		for _, match := range matches {
			candidates[match] = struct{}{}
		}
	}
	if fromPath, err := exec.LookPath("mongod.exe"); err == nil {
		if resolved, err := filepath.Abs(fromPath); err == nil {
			candidates[resolved] = struct{}{}
		}
	}

	var binaries []Binary
	for path := range candidates {
		binary, err := Probe(ctx, path)
		if err != nil {
			continue
		}
		binaries = append(binaries, binary)
	}

	sort.Slice(binaries, func(i, j int) bool {
		a, b := binaries[i], binaries[j]
		if a.Major != b.Major {
			return a.Major > b.Major
		}
		if a.Minor != b.Minor {
			return a.Minor > b.Minor
		}
		return a.Patch > b.Patch
	})
	return binaries, nil
}

// Probe runs `mongod --version` and parses the reported version.
func Probe(ctx context.Context, path string) (Binary, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.SysProcAttr = winsys.HiddenProcAttr()

	output, err := cmd.Output()
	if err != nil {
		return Binary{}, fmt.Errorf("run %q --version: %w", path, err)
	}

	match := versionPattern.FindStringSubmatch(firstLine(string(output)))
	if match == nil {
		return Binary{}, fmt.Errorf("no version in output of %q", path)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])

	return Binary{
		Path:    path,
		Version: fmt.Sprintf("%d.%d.%d", major, minor, patch),
		Major:   major,
		Minor:   minor,
		Patch:   patch,
	}, nil
}

func firstLine(text string) string {
	if index := strings.IndexAny(text, "\r\n"); index >= 0 {
		return text[:index]
	}
	return text
}
