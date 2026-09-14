package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mongoctl/internal/elevate"
	"mongoctl/internal/mongobin"
	"mongoctl/internal/mongoconf"
	"mongoctl/internal/store"
	"mongoctl/internal/winsys"
)

// WindowsServiceName is the service the MongoDB Windows installer registers.
const WindowsServiceName = "MongoDB"

// SystemService exposes settings, binary discovery and Windows service control.
type SystemService struct {
	core *Core
}

// NewSystemService binds system operations to the application core.
func NewSystemService(core *Core) *SystemService {
	return &SystemService{core: core}
}

// SystemInfo is the environment summary the UI shows on its settings screen.
type SystemInfo struct {
	Mongod   mongobin.Binary `json:"mongod"`
	Settings store.Settings  `json:"settings"`

	// Elevated reports whether the app already runs as administrator, in which
	// case service operations do not raise a UAC prompt.
	Elevated bool `json:"elevated"`

	Service      *winsys.ServiceInfo `json:"service"`
	ServiceError string              `json:"serviceError"`
}

// Info returns the current environment summary.
func (s *SystemService) Info() SystemInfo {
	info := SystemInfo{
		Mongod:   s.core.Mongod(),
		Settings: s.core.store.State().Settings,
		Elevated: winsys.IsElevated(),
	}

	service, err := winsys.QueryService(WindowsServiceName)
	if err != nil {
		info.ServiceError = err.Error()
	} else {
		info.Service = &service
	}
	return info
}

// AvailableBinaries lists every mongod installation found on the machine.
func (s *SystemService) AvailableBinaries() ([]mongobin.Binary, error) {
	binaries, err := mongobin.Discover(s.core.context())
	if err != nil {
		return nil, err
	}
	if binaries == nil {
		return []mongobin.Binary{}, nil
	}
	return binaries, nil
}

// UpdateSettings validates and stores user preferences.
func (s *SystemService) UpdateSettings(settings store.Settings) (store.Settings, error) {
	settings.DataRoot = strings.TrimSpace(settings.DataRoot)
	settings.MongodPath = strings.TrimSpace(settings.MongodPath)

	if settings.DataRoot == "" {
		return store.Settings{}, errors.New("data directory is required")
	}
	if settings.BasePort < 1024 || settings.BasePort > 65535 {
		return store.Settings{}, errors.New("base port must be between 1024 and 65535")
	}
	if settings.MongodPath != "" {
		if _, err := mongobin.Probe(s.core.context(), settings.MongodPath); err != nil {
			return store.Settings{}, fmt.Errorf("selected mongod is not usable: %w", err)
		}
	}

	if err := s.core.store.Update(func(state *store.State) error {
		state.Settings = settings
		return nil
	}); err != nil {
		return store.Settings{}, err
	}

	s.core.resolveMongod(s.core.context())
	return s.core.store.State().Settings, nil
}

// ChooseDirectory opens the native folder picker.
func (s *SystemService) ChooseDirectory(title string) (string, error) {
	return wailsruntime.OpenDirectoryDialog(s.core.context(), wailsruntime.OpenDialogOptions{
		Title: title,
	})
}

// ServiceStart starts the Windows MongoDB service, prompting for elevation.
func (s *SystemService) ServiceStart() error {
	return runPrivileged(elevate.Request{Operation: elevate.StartService, Service: WindowsServiceName})
}

// ServiceStop stops the Windows MongoDB service, prompting for elevation.
func (s *SystemService) ServiceStop() error {
	return runPrivileged(elevate.Request{Operation: elevate.StopService, Service: WindowsServiceName})
}

// ServiceRestart restarts the Windows MongoDB service, prompting for elevation.
func (s *SystemService) ServiceRestart() error {
	return runPrivileged(elevate.Request{Operation: elevate.RestartService, Service: WindowsServiceName})
}

// ServiceSetStartType changes whether the Windows service starts at boot.
func (s *SystemService) ServiceSetStartType(startType string) error {
	return runPrivileged(elevate.Request{
		Operation: elevate.SetStartType,
		Service:   WindowsServiceName,
		StartType: startType,
	})
}

// ServiceReadConfig returns the Windows service's mongod.cfg. Reading needs no
// elevation, only writing does.
func (s *SystemService) ServiceReadConfig() (string, error) {
	path, err := winsys.ServiceConfigPath(WindowsServiceName)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read service configuration %q: %w", path, err)
	}
	return string(data), nil
}

// ServiceWriteConfig replaces the Windows service's configuration. The content
// is staged in a temporary file and the privileged helper resolves the
// destination itself from the service registration.
func (s *SystemService) ServiceWriteConfig(content string) error {
	config, err := mongoconf.Parse([]byte(content))
	if err != nil {
		return err
	}
	if config.Port() == 0 {
		return errors.New("configuration must set net.port")
	}
	if config.DBPath() == "" {
		return errors.New("configuration must set storage.dbPath")
	}

	staged, err := os.CreateTemp("", "mongoctl-service-*.conf")
	if err != nil {
		return fmt.Errorf("stage configuration: %w", err)
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)

	if _, err := staged.WriteString(content); err != nil {
		staged.Close()
		return fmt.Errorf("stage configuration: %w", err)
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("stage configuration: %w", err)
	}

	return runPrivileged(elevate.Request{
		Operation: elevate.WriteServiceConf,
		Service:   WindowsServiceName,
		Source:    stagedPath,
	})
}

// runPrivileged performs an operation that needs administrator rights by
// re-launching this same executable in its privileged mode. Using the running
// binary rather than a separate helper is what keeps the application a single
// distributable file.
func runPrivileged(request elevate.Request) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate application executable: %w", err)
	}

	resultPath, cleanup, err := stageResultFile()
	if err != nil {
		return err
	}
	defer cleanup()
	request.ResultPath = resultPath

	args := request.Args()
	if winsys.IsElevated() {
		// Already an administrator, so no UAC prompt is needed.
		command := exec.Command(executable, args...)
		command.SysProcAttr = winsys.HiddenProcAttr()
		err = command.Run()
	} else {
		err = elevate.Run(executable, args)
	}

	// The elevated process has no pipe back to here, so its own message — which
	// is far more useful than an exit code — arrives through the result file.
	if message := readResultFile(resultPath); message != "" {
		return errors.New(message)
	}
	if errors.Is(err, elevate.ErrDeclined) {
		return errors.New("the operation needs administrator rights and the prompt was dismissed")
	}
	return err
}

func stageResultFile() (path string, cleanup func(), err error) {
	file, err := os.CreateTemp("", "mongoctl-result-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("prepare result file: %w", err)
	}
	name := file.Name()
	file.Close()

	return name, func() { os.Remove(name) }, nil
}

func readResultFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
