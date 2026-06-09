/*
 FILE 4: internal/env/manager.go
 Fixed:
   - PathAdd: added AppendPosition parameter (pre/post support - was silently ignored before)
   - PathAdd: checkExist bool removed from internal logic; caller decides whether to error
   - PathAddAndRefresh: updated signature to match PathAdd
   - BroadcastChange: new public method so refreshCmd in cli.go can call it
   - broadcastEnvironmentChange: now actually uses the registry helper for real WM_SETTINGCHANGE
*/

package env

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
	pkgregistry "github.com/cumulus13/pathman/pkg/registry"
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

	m.BroadcastChange()
	return nil
}

// SetAndRefresh sets a variable and updates current terminal immediately
func (m *Manager) SetAndRefresh(name, value string, scope Scope) error {
	if err := m.Set(name, value, scope); err != nil {
		return err
	}

	// Update current process
	os.Setenv(name, value)

	// Also update PATH specifically by reading combined registry values
	if strings.EqualFold(name, "PATH") {
		m.refreshCurrentPath()
	}

	// Broadcast to all windows
	rh := pkgregistry.NewRegistryHelper()
	rh.BroadcastEnvironmentChange()

	return nil
}

// BroadcastChange sends WM_SETTINGCHANGE to all windows so they pick up new env vars.
// Exposed as public so cli commands (e.g. refreshCmd) can call it directly.
func (m *Manager) BroadcastChange() error {
	rh := pkgregistry.NewRegistryHelper()
	return rh.BroadcastEnvironmentChange()
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

	m.BroadcastChange()
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
			continue
		}
		vars = append(vars, Variable{
			Name:  name,
			Value: value,
			Scope: scope,
		})
	}

	return vars, nil
}

// AppendValue appends a string to an existing variable with duplicate checking.
// position controls whether the new value is added at the front (AppendPre) or end (AppendPost).
func (m *Manager) AppendValue(name, newValue string, scope Scope, position AppendPosition) error {
	v, err := m.Get(name, scope)
	if err != nil {
		return err
	}

	currentValue := NormalizePathSeparator(v.Value)
	newValue = strings.TrimSpace(newValue)

	// Check if value already exists (case-insensitive)
	parts := strings.Split(currentValue, string(os.PathListSeparator))
	for _, p := range parts {
		if strings.EqualFold(strings.TrimSpace(p), newValue) {
			return fmt.Errorf("value already exists in %s", name)
		}
	}

	var newFullValue string
	switch position {
	case AppendPre:
		if currentValue == "" {
			newFullValue = newValue
		} else {
			newFullValue = newValue + string(os.PathListSeparator) + currentValue
		}
	default: // AppendPost
		if currentValue == "" {
			newFullValue = newValue
		} else {
			newFullValue = currentValue + string(os.PathListSeparator) + newValue
		}
	}

	newFullValue = NormalizePathSeparator(newFullValue)
	return m.Set(name, newFullValue, scope)
}

// AppendValueAndRefresh appends and updates current terminal session immediately
func (m *Manager) AppendValueAndRefresh(name, newValue string, scope Scope, position AppendPosition) error {
	if err := m.AppendValue(name, newValue, scope, position); err != nil {
		return err
	}

	// Get the updated value
	v, err := m.Get(name, scope)
	if err != nil {
		return err
	}

	// Update current process environment
	os.Setenv(name, v.Value)

	// Also update PATH specifically
	if strings.EqualFold(name, "PATH") {
		m.refreshCurrentPath()
	}

	// Broadcast to all windows
	rh := pkgregistry.NewRegistryHelper()
	rh.BroadcastEnvironmentChange()

	return nil
}

// PathAdd adds a directory to a PATH-like variable.
//
// position: AppendPre adds to the front, AppendPost adds to the end.
// checkExist: if true, returns an error when newPath does not exist on disk.
//
// NOTE: the cli layer already prints a warning for non-existent paths before
// calling PathAdd, so it passes checkExist=false to avoid a redundant error.
func (m *Manager) PathAdd(pathVar, newPath string, scope Scope, position AppendPosition, checkExist bool) error {
	if checkExist {
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", newPath)
		}
	}

	v, err := m.Get(pathVar, scope)
	if err != nil {
		return err
	}

	paths := strings.Split(v.Value, string(os.PathListSeparator))

	// Check for duplicate (case-insensitive)
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(newPath)) {
			return fmt.Errorf("path already exists in %s", pathVar)
		}
	}

	var newPaths []string
	switch position {
	case AppendPre:
		newPaths = append([]string{newPath}, paths...)
	default: // AppendPost
		newPaths = append(paths, newPath)
	}

	newValue := NormalizePathSeparator(strings.Join(newPaths, string(os.PathListSeparator)))
	return m.Set(pathVar, newValue, scope)
}

// PathAddAndRefresh adds to PATH and updates current terminal immediately.
// Signature matches PathAdd (position + checkExist).
func (m *Manager) PathAddAndRefresh(pathVar, newPath string, scope Scope, position AppendPosition, checkExist bool) error {
	if err := m.PathAdd(pathVar, newPath, scope, position, checkExist); err != nil {
		return err
	}

	v, err := m.Get(pathVar, scope)
	if err != nil {
		return err
	}

	// Update current process PATH
	os.Setenv(pathVar, v.Value)

	// Also rebuild combined PATH
	if strings.EqualFold(pathVar, "PATH") {
		m.refreshCurrentPath()
	}

	// Broadcast
	rh := pkgregistry.NewRegistryHelper()
	rh.BroadcastEnvironmentChange()

	return nil
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
		if strings.TrimSpace(p) != "" {
			newPaths = append(newPaths, p)
		}
	}

	if !found {
		return fmt.Errorf("path not found in %s", pathVar)
	}

	newValue := NormalizePathSeparator(strings.Join(newPaths, string(os.PathListSeparator)))
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

// refreshCurrentPath rebuilds the current process PATH from registry (System + User)
func (m *Manager) refreshCurrentPath() {
	sysPath, err := m.Get("PATH", ScopeSystem)
	if err != nil {
		sysPath = &Variable{Value: ""}
	}

	userPath, err := m.Get("PATH", ScopeUser)
	if err != nil {
		userPath = &Variable{Value: ""}
	}

	combined := sysPath.Value
	if userPath.Value != "" {
		if combined != "" {
			combined += ";" + userPath.Value
		} else {
			combined = userPath.Value
		}
	}

	os.Setenv("PATH", combined)
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
