//go:build windows

package winsys

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcessGroup is a Job Object that owns every mongod this application spawns.
// If the GUI dies for any reason the kernel terminates the whole group, so a
// crash can never leave orphaned servers holding ports and data-directory locks.
type ProcessGroup struct {
	handle windows.Handle

	// inherited reports whether the current process itself joined the job. When
	// it did, children inherit membership at creation time and Adopt is a no-op;
	// otherwise each child must be assigned explicitly after it starts.
	inherited bool
}

// NewProcessGroup creates the job and tries to place the current process in it.
func NewProcessGroup() (*ProcessGroup, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}

	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("configure job object: %w", err)
	}

	group := &ProcessGroup{handle: handle}
	if err := windows.AssignProcessToJobObject(handle, windows.CurrentProcess()); err == nil {
		group.inherited = true
	}
	return group, nil
}

// Adopt places an already-started process into the group. It is only needed when
// the current process could not join the job itself.
func (g *ProcessGroup) Adopt(pid int) error {
	if g == nil || g.inherited {
		return nil
	}
	handle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(pid),
	)
	if err != nil {
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	if err := windows.AssignProcessToJobObject(g.handle, handle); err != nil {
		return fmt.Errorf("assign process %d to job: %w", pid, err)
	}
	return nil
}

// Close releases the job handle, which terminates every process still inside it.
func (g *ProcessGroup) Close() error {
	if g == nil || g.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(g.handle)
	g.handle = 0
	return err
}
