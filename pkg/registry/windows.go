package registry

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	// Windows API constants for environment variable broadcasting
	HWND_BROADCAST   = uintptr(0xFFFF)
	WM_SETTINGCHANGE = uint32(0x001A)
	SMTO_ABORTIFHUNG = uint32(0x0002)
)

var (
	user32DLL           = syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeoutW = user32DLL.NewProc("SendMessageTimeoutW")
	kernel32DLL              = syscall.NewLazyDLL("kernel32.dll")
	procSetEnvironmentVariableW = kernel32DLL.NewProc("SetEnvironmentVariableW")
)

// Environment keys for different scopes
const (
	UserEnvironmentKey   = `Environment`
	SystemEnvironmentKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
)

// RegistryHelper provides additional Windows registry utilities
type RegistryHelper struct{}

// NewRegistryHelper creates a new registry helper
func NewRegistryHelper() *RegistryHelper {
	return &RegistryHelper{}
}

// OpenUserEnvironment opens the user environment registry key
func (rh *RegistryHelper) OpenUserEnvironment(access uint32) (registry.Key, error) {
	return registry.OpenKey(registry.CURRENT_USER, UserEnvironmentKey, access)
}

// OpenSystemEnvironment opens the system environment registry key
func (rh *RegistryHelper) OpenSystemEnvironment(access uint32) (registry.Key, error) {
	return registry.OpenKey(registry.LOCAL_MACHINE, SystemEnvironmentKey, access)
}

// BroadcastEnvironmentChange notifies all windows about environment variable changes
// This is equivalent to what setx does internally
func (rh *RegistryHelper) BroadcastEnvironmentChange() error {
	// Prepare the "Environment" string as UTF-16
	envStr, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return fmt.Errorf("failed to convert string: %w", err)
	}

	// Send WM_SETTINGCHANGE message to all top-level windows
	// This tells Explorer and other apps to reload environment variables
	ret, _, lastErr := procSendMessageTimeoutW.Call(
		HWND_BROADCAST,
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envStr)),
		uintptr(SMTO_ABORTIFHUNG),
		5000, // 5 second timeout
		uintptr(0), // No result needed
	)

	if ret == 0 {
		return fmt.Errorf("SendMessageTimeout failed: %v", lastErr)
	}

	return nil
}

// UpdateProcessEnvironment updates the current process environment
func (rh *RegistryHelper) UpdateProcessEnvironment(name, value string) error {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("failed to convert name: %w", err)
	}

	valuePtr, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return fmt.Errorf("failed to convert value: %w", err)
	}

	ret, _, lastErr := procSetEnvironmentVariableW.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(valuePtr)),
	)

	if ret == 0 {
		return fmt.Errorf("SetEnvironmentVariable failed: %v", lastErr)
	}

	return nil
}

// GetSystemEnvironmentVariable retrieves a system environment variable
func (rh *RegistryHelper) GetSystemEnvironmentVariable(name string) (string, error) {
	key, err := rh.OpenSystemEnvironment(registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("failed to open system environment: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(name)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", fmt.Errorf("system variable '%s' not found", name)
		}
		return "", fmt.Errorf("failed to read system variable: %w", err)
	}

	return value, nil
}

// GetUserEnvironmentVariable retrieves a user environment variable
func (rh *RegistryHelper) GetUserEnvironmentVariable(name string) (string, error) {
	key, err := rh.OpenUserEnvironment(registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("failed to open user environment: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(name)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", fmt.Errorf("user variable '%s' not found", name)
		}
		return "", fmt.Errorf("failed to read user variable: %w", err)
	}

	return value, nil
}

// SetUserEnvironmentVariable sets a user environment variable
func (rh *RegistryHelper) SetUserEnvironmentVariable(name, value string) error {
	key, err := rh.OpenUserEnvironment(registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open user environment for writing: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("failed to set user variable: %w", err)
	}

	return rh.BroadcastEnvironmentChange()
}

// SetSystemEnvironmentVariable sets a system environment variable
func (rh *RegistryHelper) SetSystemEnvironmentVariable(name, value string) error {
	// Check for admin privileges
	if !rh.IsAdmin() {
		return fmt.Errorf("administrator privileges required to modify system environment variables")
	}

	key, err := rh.OpenSystemEnvironment(registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open system environment for writing: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("failed to set system variable: %w", err)
	}

	return rh.BroadcastEnvironmentChange()
}

// DeleteUserEnvironmentVariable deletes a user environment variable
func (rh *RegistryHelper) DeleteUserEnvironmentVariable(name string) error {
	key, err := rh.OpenUserEnvironment(registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open user environment for writing: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(name); err != nil {
		if err == registry.ErrNotExist {
			return fmt.Errorf("user variable '%s' not found", name)
		}
		return fmt.Errorf("failed to delete user variable: %w", err)
	}

	return rh.BroadcastEnvironmentChange()
}

// DeleteSystemEnvironmentVariable deletes a system environment variable
func (rh *RegistryHelper) DeleteSystemEnvironmentVariable(name string) error {
	if !rh.IsAdmin() {
		return fmt.Errorf("administrator privileges required to modify system environment variables")
	}

	key, err := rh.OpenSystemEnvironment(registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open system environment for writing: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(name); err != nil {
		if err == registry.ErrNotExist {
			return fmt.Errorf("system variable '%s' not found", name)
		}
		return fmt.Errorf("failed to delete system variable: %w", err)
	}

	return rh.BroadcastEnvironmentChange()
}

// ListUserEnvironmentVariables lists all user environment variables
func (rh *RegistryHelper) ListUserEnvironmentVariables() (map[string]string, error) {
	key, err := rh.OpenUserEnvironment(registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("failed to open user environment: %w", err)
	}
	defer key.Close()

	return rh.enumValues(key)
}

// ListSystemEnvironmentVariables lists all system environment variables
func (rh *RegistryHelper) ListSystemEnvironmentVariables() (map[string]string, error) {
	key, err := rh.OpenSystemEnvironment(registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("failed to open system environment: %w", err)
	}
	defer key.Close()

	return rh.enumValues(key)
}

// IsAdmin checks if the current process has administrator privileges
func (rh *RegistryHelper) IsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}

	return member
}

// ExpandEnvironmentStrings expands environment variables in a string
func (rh *RegistryHelper) ExpandEnvironmentStrings(input string) (string, error) {
	// First call to get required buffer size
	n, err := windows.ExpandEnvironmentStrings(syscall.StringToUTF16Ptr(input), nil, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get buffer size: %w", err)
	}

	if n == 0 {
		return input, nil
	}

	// Allocate buffer and get expanded string
	buf := make([]uint16, n)
	_, err = windows.ExpandEnvironmentStrings(syscall.StringToUTF16Ptr(input), &buf[0], n)
	if err != nil {
		return "", fmt.Errorf("failed to expand environment strings: %w", err)
	}

	return syscall.UTF16ToString(buf), nil
}

// GetEffectivePath returns the combined user and system PATH
func (rh *RegistryHelper) GetEffectivePath() (string, error) {
	userPath, err := rh.GetUserEnvironmentVariable("PATH")
	if err != nil {
		userPath = ""
	}

	systemPath, err := rh.GetSystemEnvironmentVariable("PATH")
	if err != nil {
		return "", fmt.Errorf("failed to get system PATH: %w", err)
	}

	if userPath != "" {
		return systemPath + ";" + userPath, nil
	}
	return systemPath, nil
}

// ValidateRegistryPath checks if a registry path exists and is accessible
func (rh *RegistryHelper) ValidateRegistryPath(path string) error {
	// This validates that we can actually access the Windows registry
	// Try to open a known key to verify registry access
	_, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("cannot access Windows registry: %w", err)
	}
	return nil
}

// enumValues is a helper to enumerate registry values
func (rh *RegistryHelper) enumValues(key registry.Key) (map[string]string, error) {
	values := make(map[string]string)
	
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("failed to read value names: %w", err)
	}

	for _, name := range names {
		value, _, err := key.GetStringValue(name)
		if err != nil {
			// Skip binary or unreadable values
			continue
		}
		values[name] = value
	}

	return values, nil
}

// BackupEnvironmentVariable creates a backup of an environment variable
func (rh *RegistryHelper) BackupEnvironmentVariable(name string, scope string) (string, error) {
	var value string
	var err error

	switch scope {
	case "user":
		value, err = rh.GetUserEnvironmentVariable(name)
	case "system":
		value, err = rh.GetSystemEnvironmentVariable(name)
	default:
		return "", fmt.Errorf("invalid scope: %s", scope)
	}

	if err != nil {
		return "", err
	}

	// Create backup variable with _BACKUP suffix
	backupName := name + "_PATHMAN_BACKUP"
	switch scope {
	case "user":
		err = rh.SetUserEnvironmentVariable(backupName, value)
	case "system":
		err = rh.SetSystemEnvironmentVariable(backupName, value)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupName, nil
}

// RestoreEnvironmentVariable restores an environment variable from backup
func (rh *RegistryHelper) RestoreEnvironmentVariable(name string, scope string) error {
	backupName := name + "_PATHMAN_BACKUP"
	
	var backupValue string
	var err error

	switch scope {
	case "user":
		backupValue, err = rh.GetUserEnvironmentVariable(backupName)
		if err != nil {
			return fmt.Errorf("no backup found for %s", name)
		}
		err = rh.SetUserEnvironmentVariable(name, backupValue)
		// Clean up backup
		rh.DeleteUserEnvironmentVariable(backupName)
	case "system":
		backupValue, err = rh.GetSystemEnvironmentVariable(backupName)
		if err != nil {
			return fmt.Errorf("no backup found for %s", name)
		}
		err = rh.SetSystemEnvironmentVariable(name, backupValue)
		// Clean up backup
		rh.DeleteSystemEnvironmentVariable(backupName)
	}

	if err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	return nil
}