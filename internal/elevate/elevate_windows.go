//go:build windows

// Package elevate runs a helper executable under a UAC prompt.
//
// Elevation is per action: each call spawns a short-lived privileged process
// that performs one operation and exits. Nothing privileged stays resident and
// there is no IPC channel for a lower-privileged process to talk to, which
// removes the local privilege-escalation surface a persistent elevated service
// would introduce. The cost is a UAC prompt per operation.
package elevate

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrDeclined reports that the user dismissed the UAC prompt.
var ErrDeclined = errors.New("elevation was declined")

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	swHide                = 0
)

var (
	shell32          = windows.NewLazySystemDLL("shell32.dll")
	procShellExecute = shell32.NewProc("ShellExecuteExW")
)

// shellExecuteInfo mirrors SHELLEXECUTEINFOW. Field order and padding must match
// the Win32 struct exactly; cbSize is validated by the API.
type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           windows.Handle
	verb           *uint16
	file           *uint16
	parameters     *uint16
	directory      *uint16
	show           int32
	instApp        windows.Handle
	idList         uintptr
	class          *uint16
	keyClass       windows.Handle
	hotKey         uint32
	iconOrMonitor  windows.Handle
	process        windows.Handle
}

// Run launches exe elevated with the given arguments and waits for it to finish.
func Run(exe string, args []string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return fmt.Errorf("encode verb: %w", err)
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("encode executable path: %w", err)
	}

	var parameters *uint16
	if len(args) > 0 {
		parameters, err = windows.UTF16PtrFromString(buildCommandLine(args))
		if err != nil {
			return fmt.Errorf("encode arguments: %w", err)
		}
	}

	info := shellExecuteInfo{
		fMask:      seeMaskNoCloseProcess | seeMaskNoAsync,
		verb:       verb,
		file:       file,
		parameters: parameters,
		show:       swHide,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))

	ret, _, callErr := procShellExecute.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == windows.ERROR_CANCELLED {
			return ErrDeclined
		}
		return fmt.Errorf("launch elevated helper: %w", callErr)
	}
	if info.process == 0 {
		return errors.New("elevated helper did not report a process handle")
	}
	defer windows.CloseHandle(info.process)

	if _, err := windows.WaitForSingleObject(info.process, windows.INFINITE); err != nil {
		return fmt.Errorf("wait for elevated helper: %w", err)
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.process, &exitCode); err != nil {
		return fmt.Errorf("read helper exit code: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("elevated helper failed with exit code %d", exitCode)
	}
	return nil
}

// buildCommandLine joins arguments using the quoting rules CommandLineToArgvW
// applies. No shell is involved, so this is purely about argument boundaries.
func buildCommandLine(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteArg(arg))
	}
	return strings.Join(quoted, " ")
}

func quoteArg(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}

	var builder strings.Builder
	builder.WriteByte('"')

	for index := 0; index < len(arg); {
		slashes := 0
		for index < len(arg) && arg[index] == '\\' {
			slashes++
			index++
		}

		if index == len(arg) {
			// Trailing backslashes must not escape the closing quote.
			builder.WriteString(strings.Repeat(`\`, slashes*2))
			break
		}

		if arg[index] == '"' {
			builder.WriteString(strings.Repeat(`\`, slashes*2+1))
		} else {
			builder.WriteString(strings.Repeat(`\`, slashes))
		}
		builder.WriteByte(arg[index])
		index++
	}

	builder.WriteByte('"')
	return builder.String()
}
