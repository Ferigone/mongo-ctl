//go:build windows

package elevate

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"mongoctl/internal/mongoconf"
	"mongoctl/internal/winsys"
)

// OperationFlag marks a process started to perform one privileged operation
// instead of showing the UI. Keeping this in the main binary is what lets the
// application ship as a single file: it re-launches itself elevated rather than
// carrying a separate helper executable that would have to be distributed,
// found at runtime and trusted.
const OperationFlag = "--privileged-operation"

const operationTimeout = 60 * time.Second

// Operation is one of the privileged actions the UI can request.
type Operation string

const (
	StartService     Operation = "start-service"
	StopService      Operation = "stop-service"
	RestartService   Operation = "restart-service"
	SetStartType     Operation = "set-start-type"
	WriteServiceConf Operation = "write-service-config"
)

// Request describes a privileged operation to perform.
type Request struct {
	Operation Operation
	Service   string

	// StartType is required by SetStartType.
	StartType string

	// Source is the staged replacement file used by WriteServiceConf. The
	// destination is deliberately absent: the elevated side derives it from the
	// service registration, so this process cannot be aimed at another file.
	Source string

	// ResultPath receives the failure message, since an elevated process started
	// through ShellExecute has no pipes back to its parent.
	ResultPath string
}

// Args renders the request as command-line arguments.
func (r Request) Args() []string {
	args := []string{
		OperationFlag,
		"-operation", string(r.Operation),
		"-service", r.Service,
		"-result", r.ResultPath,
	}
	if r.StartType != "" {
		args = append(args, "-start-type", r.StartType)
	}
	if r.Source != "" {
		args = append(args, "-source", r.Source)
	}
	return args
}

// Requested reports whether this process was started to perform an operation
// rather than to show the UI.
func Requested() bool {
	return len(os.Args) > 1 && os.Args[1] == OperationFlag
}

// Execute performs the requested operation and returns a process exit code. Any
// failure is also written to the result file so the parent can report it.
func Execute() int {
	set := flag.NewFlagSet("privileged", flag.ContinueOnError)
	set.SetOutput(io.Discard)

	operation := set.String("operation", "", "")
	service := set.String("service", "", "")
	startType := set.String("start-type", "", "")
	source := set.String("source", "", "")
	result := set.String("result", "", "")

	if err := set.Parse(os.Args[2:]); err != nil {
		return 2
	}

	err := perform(Operation(*operation), *service, *startType, *source)
	if err == nil {
		return 0
	}

	if *result != "" {
		os.WriteFile(*result, []byte(err.Error()), 0o600)
	}
	return 1
}

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,80}$`)

func perform(operation Operation, service, startType, source string) error {
	if !serviceNamePattern.MatchString(service) {
		return fmt.Errorf("invalid service name %q", service)
	}

	switch operation {
	case StartService:
		return winsys.StartService(service, operationTimeout)

	case StopService:
		return winsys.StopService(service, operationTimeout)

	case RestartService:
		if err := winsys.StopService(service, operationTimeout); err != nil {
			return err
		}
		return winsys.StartService(service, operationTimeout)

	case SetStartType:
		return applyStartType(service, startType)

	case WriteServiceConf:
		return writeServiceConfig(service, source)

	default:
		return fmt.Errorf("unsupported operation %q", operation)
	}
}

func applyStartType(service, startType string) error {
	switch winsys.ServiceStartType(startType) {
	case winsys.StartAutomatic, winsys.StartManual, winsys.StartDisabled:
		return winsys.SetServiceStartType(service, winsys.ServiceStartType(startType))
	default:
		return fmt.Errorf("invalid start type %q", startType)
	}
}

func writeServiceConfig(service, source string) error {
	if source == "" {
		return errors.New("no source file given")
	}

	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read replacement configuration: %w", err)
	}

	// Reject anything unusable before it can replace a working configuration.
	config, err := mongoconf.Parse(content)
	if err != nil {
		return err
	}
	if config.Port() == 0 {
		return errors.New("configuration must set net.port")
	}
	if config.DBPath() == "" {
		return errors.New("configuration must set storage.dbPath")
	}

	target, err := winsys.ServiceConfigPath(service)
	if err != nil {
		return err
	}

	if existing, err := os.ReadFile(target); err == nil {
		if err := os.WriteFile(target+".mongoctl.bak", existing, 0o644); err != nil {
			return fmt.Errorf("back up existing configuration: %w", err)
		}
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("write configuration %q: %w", target, err)
	}
	return nil
}
