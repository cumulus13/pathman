package env

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Manager handles environment variable operations
type Manager struct {
	dryRun bool
}

// NewManager creates a new environment variable manager
func NewManager(dryRun bool) *Manager {
	return &Manager{
		dryRun: dryRun,
	}
}

// Get retrieves an environment variable
func (m *Manager) Get(name string, scope Scope) (*Variable, error) {
	k, err := m.openKey(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to open registry: %w", err)
	}
	defer k.Close()

	value, _, err := k.GetStringValue(name)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil, fmt.Errorf("variable '%s' not found in %s scope", name, scope)
		}
		return nil, fmt.Errorf("failed to read variable: %w", err)
	}

	return &Variable{
		Name:  name,
		Value: value,
		Scope: scope,
	}, nil
}

// Set sets an environment variable
func (m *Manager) Set(name, value string, scope Scope) error {
	if err := validateVariableName(name); err != nil {
		return err
	}

	if m.dryRun {
		return nil
	}

	k, err := m.openKeyWrite(scope)
	if err != nil {
		return fmt.Errorf("failed to open registry for writing: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue(name, value); err != nil {
		return fmt.Errorf("failed to set variable: %w", err)
	}

	// Broadcast change
	broadcastEnvironmentChange()
	return nil
}

// Delete deletes an environment variable
func (m *Manager) Delete(name string, scope Scope) error {
	if m.dryRun {
		return nil
	}

	k, err := m.openKeyWrite(scope)
	if err != nil {
		return fmt.Errorf("failed to open registry for writing: %w", err)
	}
	defer k.Close()

	if err := k.DeleteValue(name); err != nil {
		if err == registry.ErrNotExist {
			return fmt.Errorf("variable '%s' not found in %s scope", name, scope)
		}
		return fmt.Errorf("failed to delete variable: %w", err)
	}

	broadcastEnvironmentChange()
	return nil
}

// List returns all environment variables in a scope
func (m *Manager) List(scope Scope) ([]Variable, error) {
	k, err := m.openKey(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to open registry: %w", err)
	}
	defer k.Close()

	names, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("failed to read value names: %w", err)
	}

	var vars []Variable
	for _, name := range names {
		value, _, err := k.GetStringValue(name)
		if err != nil {
			continue // Skip unreadable values
		}
		vars = append(vars, Variable{
			Name:  name,
			Value: value,
			Scope: scope,
		})
	}

	return vars, nil
}

// PathAdd adds a directory to a PATH-like variable
func (m *Manager) PathAdd(pathVar, newPath string, scope Scope, checkExist bool) error {
	v, err := m.Get(pathVar, scope)
	if err != nil {
		return err
	}

	paths := strings.Split(v.Value, string(os.PathListSeparator))
	
	if checkExist {
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", newPath)
		}
	}

	// Check if path already exists
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(newPath)) {
			return fmt.Errorf("path already exists in %s", pathVar)
		}
	}

	paths = append(paths, newPath)
	newValue := strings.Join(paths, string(os.PathListSeparator))

	return m.Set(pathVar, newValue, scope)
}

// PathRemove removes a directory from a PATH-like variable
func (m *Manager) PathRemove(pathVar, removePath string, scope Scope) error {
	v, err := m.Get(pathVar, scope)
	if err != nil {
		return err
	}

	paths := strings.Split(v.Value, string(os.PathListSeparator))
	var newPaths []string
	found := false

	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(removePath)) {
			found = true
			continue
		}
		newPaths = append(newPaths, p)
	}

	if !found {
		return fmt.Errorf("path not found in %s", pathVar)
	}

	newValue := strings.Join(newPaths, string(os.PathListSeparator))
	return m.Set(pathVar, newValue, scope)
}

// PathList lists all entries in a PATH-like variable
func (m *Manager) PathList(pathVar string, scope Scope) ([]PathEntry, error) {
	v, err := m.Get(pathVar, scope)
	if err != nil {
		return nil, err
	}

	paths := strings.Split(v.Value, string(os.PathListSeparator))
	var entries []PathEntry

	for i, p := range paths {
		if p == "" {
			continue
		}
		exist := false
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			exist = true
		}
		entries = append(entries, PathEntry{
			Index: i,
			Value: p,
			Exist: exist,
		})
	}

	return entries, nil
}

func (m *Manager) openKey(scope Scope) (registry.Key, error) {
	switch scope {
	case ScopeUser:
		return registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	case ScopeSystem:
		return registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
	default:
		return registry.Key(0), fmt.Errorf("invalid scope: %v", scope)
	}
}

func (m *Manager) openKeyWrite(scope Scope) (registry.Key, error) {
	switch scope {
	case ScopeUser:
		return registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	case ScopeSystem:
		return registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.SET_VALUE)
	default:
		return registry.Key(0), fmt.Errorf("invalid scope: %v", scope)
	}
}

func validateVariableName(name string) error {
	if name == "" {
		return &ValidationError{Field: "name", Message: "variable name cannot be empty"}
	}
	if strings.Contains(name, "=") {
		return &ValidationError{Field: "name", Message: "variable name cannot contain '='"}
	}
	return nil
}

// broadcastEnvironmentChange notifies the system about environment changes
func broadcastEnvironmentChange() {
	// Send WM_SETTINGCHANGE message to all windows
	// This is handled by the syscall in the Windows API
	// The actual implementation would use:
	// SendMessageTimeoutW(HWND_BROADCAST, WM_SETTINGCHANGE, 0, "Environment", SMTO_ABORTIFHUNG, 5000)
	
	// For now, we'll just update the current process
	os.Setenv("PATH", os.Getenv("PATH"))
}