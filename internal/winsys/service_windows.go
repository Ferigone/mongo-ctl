//go:build windows

package winsys

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// ErrServiceNotFound reports that no service with the requested name is registered.
var ErrServiceNotFound = errors.New("service not found")

// ServiceState mirrors the Windows service control manager states the UI cares about.
type ServiceState string

const (
	ServiceRunning      ServiceState = "running"
	ServiceStopped      ServiceState = "stopped"
	ServiceStartPending ServiceState = "start_pending"
	ServiceStopPending  ServiceState = "stop_pending"
	ServicePaused       ServiceState = "paused"
	ServiceUnknown      ServiceState = "unknown"
)

// ServiceStartType is how the service starts at boot.
type ServiceStartType string

const (
	StartAutomatic ServiceStartType = "automatic"
	StartManual    ServiceStartType = "manual"
	StartDisabled  ServiceStartType = "disabled"
	StartUnknown   ServiceStartType = "unknown"
)

// ServiceInfo describes a registered Windows service.
type ServiceInfo struct {
	Name        string           `json:"name"`
	DisplayName string           `json:"displayName"`
	State       ServiceState     `json:"state"`
	StartType   ServiceStartType `json:"startType"`
	BinaryPath  string           `json:"binaryPath"`
}

// QueryService reads a service's state and configuration. It needs no elevation.
func QueryService(name string) (ServiceInfo, error) {
	manager, err := connectManager(windows.SC_MANAGER_CONNECT)
	if err != nil {
		return ServiceInfo{}, err
	}
	defer windows.CloseServiceHandle(manager.Handle)

	service, err := openService(manager, name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return ServiceInfo{}, err
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return ServiceInfo{}, fmt.Errorf("query service %q: %w", name, err)
	}
	config, err := service.Config()
	if err != nil {
		return ServiceInfo{}, fmt.Errorf("read config of service %q: %w", name, err)
	}

	return ServiceInfo{
		Name:        name,
		DisplayName: config.DisplayName,
		State:       mapServiceState(status.State),
		StartType:   mapStartType(config.StartType),
		BinaryPath:  config.BinaryPathName,
	}, nil
}

// StartService starts a stopped service and waits until it reports running.
// Requires elevation; call it from the privileged helper.
func StartService(name string, timeout time.Duration) error {
	manager, err := connectManager(windows.SC_MANAGER_CONNECT)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(manager.Handle)

	service, err := openService(manager, name, windows.SERVICE_START|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer service.Close()

	if err := service.Start(); err != nil {
		return fmt.Errorf("start service %q: %w", name, err)
	}
	return waitForState(service, svc.Running, timeout)
}

// StopService stops a running service and waits until it reports stopped.
// Requires elevation; call it from the privileged helper.
func StopService(name string, timeout time.Duration) error {
	manager, err := connectManager(windows.SC_MANAGER_CONNECT)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(manager.Handle)

	service, err := openService(manager, name, windows.SERVICE_STOP|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer service.Close()

	if _, err := service.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop service %q: %w", name, err)
	}
	return waitForState(service, svc.Stopped, timeout)
}

// SetServiceStartType changes whether the service starts at boot.
// Requires elevation; call it from the privileged helper.
func SetServiceStartType(name string, startType ServiceStartType) error {
	manager, err := connectManager(windows.SC_MANAGER_CONNECT)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(manager.Handle)

	service, err := openService(manager, name, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_CHANGE_CONFIG)
	if err != nil {
		return err
	}
	defer service.Close()

	config, err := service.Config()
	if err != nil {
		return fmt.Errorf("read config of service %q: %w", name, err)
	}

	switch startType {
	case StartAutomatic:
		config.StartType = mgr.StartAutomatic
	case StartManual:
		config.StartType = mgr.StartManual
	case StartDisabled:
		config.StartType = mgr.StartDisabled
	default:
		return fmt.Errorf("unsupported start type %q", startType)
	}

	if err := service.UpdateConfig(config); err != nil {
		return fmt.Errorf("update config of service %q: %w", name, err)
	}
	return nil
}

// configPathPattern extracts the --config argument from a service command line,
// with or without surrounding quotes.
var configPathPattern = regexp.MustCompile(`--config\s+(?:"([^"]+)"|(\S+))`)

// ServiceConfigPath returns the mongod.conf a service was registered to use.
//
// The path is derived from the service registration rather than accepted from a
// caller, so a privileged write can only ever target that service's own config.
func ServiceConfigPath(name string) (string, error) {
	info, err := QueryService(name)
	if err != nil {
		return "", err
	}

	match := configPathPattern.FindStringSubmatch(info.BinaryPath)
	if match == nil {
		return "", fmt.Errorf("service %q does not use a --config file", name)
	}
	if match[1] != "" {
		return match[1], nil
	}
	return match[2], nil
}

func connectManager(access uint32) (*mgr.Mgr, error) {
	handle, err := windows.OpenSCManager(nil, nil, access)
	if err != nil {
		return nil, fmt.Errorf("open service control manager: %w", err)
	}
	return &mgr.Mgr{Handle: handle}, nil
}

func openService(manager *mgr.Mgr, name string, access uint32) (*mgr.Service, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("invalid service name %q: %w", name, err)
	}
	handle, err := windows.OpenService(manager.Handle, namePtr, access)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, name)
		}
		return nil, fmt.Errorf("open service %q: %w", name, err)
	}
	return &mgr.Service{Name: name, Handle: handle}, nil
}

func waitForState(service *mgr.Service, want svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := service.Query()
		if err != nil {
			return fmt.Errorf("query service %q: %w", service.Name, err)
		}
		if status.State == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service %q did not reach state %v within %s", service.Name, want, timeout)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func mapServiceState(state svc.State) ServiceState {
	switch state {
	case svc.Running:
		return ServiceRunning
	case svc.Stopped:
		return ServiceStopped
	case svc.StartPending:
		return ServiceStartPending
	case svc.StopPending:
		return ServiceStopPending
	case svc.Paused:
		return ServicePaused
	default:
		return ServiceUnknown
	}
}

func mapStartType(startType uint32) ServiceStartType {
	switch startType {
	case mgr.StartAutomatic:
		return StartAutomatic
	case mgr.StartManual:
		return StartManual
	case mgr.StartDisabled:
		return StartDisabled
	default:
		return StartUnknown
	}
}
