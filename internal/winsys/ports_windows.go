//go:build windows

package winsys

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

// PortRange is an inclusive range of TCP ports.
type PortRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Contains reports whether port falls inside the range.
func (r PortRange) Contains(port int) bool {
	return port >= r.Start && port <= r.End
}

// PortFree reports whether a port can be bound on the loopback interface right
// now. A caller may still lose the port to another process before it starts
// mongod; treat this as a pre-flight check, not a reservation.
func PortFree(port int) bool {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// ExcludedPortRanges returns TCP ranges the OS has reserved for Hyper-V, WSL or
// Docker. Binding inside one fails with a permission error that names no
// culprit, so ports are screened against this list before being offered.
func ExcludedPortRanges(ctx context.Context) ([]PortRange, error) {
	cmd := exec.CommandContext(ctx, "netsh", "int", "ipv4", "show", "excludedportrange", "protocol=tcp")
	cmd.SysProcAttr = HiddenProcAttr()

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("query excluded port ranges: %w", err)
	}
	return parseExcludedPortRanges(string(output)), nil
}

func parseExcludedPortRanges(output string) []PortRange {
	var ranges []PortRange
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		start, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		end, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		ranges = append(ranges, PortRange{Start: start, End: end})
	}
	return ranges
}

// FindFreePort returns the first bindable port at or above start, skipping ports
// in reserved, ports inside OS exclusions, and ports already bound.
func FindFreePort(start int, reserved map[int]bool, excluded []PortRange) (int, error) {
	const maxPort = 65535

	for port := start; port <= maxPort; port++ {
		if reserved[port] {
			continue
		}
		if slicesContainsPort(excluded, port) {
			continue
		}
		if PortFree(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free TCP port at or above %d", start)
}

func slicesContainsPort(ranges []PortRange, port int) bool {
	for _, r := range ranges {
		if r.Contains(port) {
			return true
		}
	}
	return false
}
