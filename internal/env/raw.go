/*
 FILE: internal/env/raw.go

 Raw registry access used by backup/restore: values are read and written with
 their registry TYPE preserved (REG_SZ vs REG_EXPAND_SZ).
*/

package env

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// RawValue is one registry value with its type.
type RawValue struct {
	Name   string
	Type   uint32 // registry.SZ, registry.EXPAND_SZ, ...
	String string // for SZ / EXPAND_SZ
	Data   []byte // for every other type
}

// Snapshot reads every value of a scope with its type.
func (m *Manager) Snapshot(scope Scope) ([]RawValue, error) {
	k, err := m.openKey(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to open registry: %w", err)
	}
	defer k.Close()

	names, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate values: %w", err)
	}

	out := make([]RawValue, 0, len(names))
	for _, name := range names {
		n, vt, err := k.GetValue(name, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to read %q: %w", name, err)
		}
		rv := RawValue{Name: name, Type: vt}
		switch vt {
		case registry.SZ, registry.EXPAND_SZ:
			str, _, err := k.GetStringValue(name)
			if err != nil {
				return nil, fmt.Errorf("failed to read %q: %w", name, err)
			}
			rv.String = str
		default:
			buf := make([]byte, n)
			if n > 0 {
				got, _, err := k.GetValue(name, buf)
				if err != nil {
					return nil, fmt.Errorf("failed to read %q: %w", name, err)
				}
				buf = buf[:got]
			}
			rv.Data = buf
		}
		out = append(out, rv)
	}
	return out, nil
}

// RestoreValue writes a string value back with the given type.
// It honours dry-run and the BeforeWrite hook like every other write.
func (m *Manager) RestoreValue(name, value string, expand bool, scope Scope) error {
	if err := validateVariableName(name); err != nil {
		return err
	}
	if m.dryRun {
		return nil
	}
	if err := m.beforeWrite(scope); err != nil {
		return err
	}

	k, err := m.openKeyWrite(scope)
	if err != nil {
		return fmt.Errorf("failed to open registry for writing: %w", err)
	}
	defer k.Close()

	if expand {
		err = k.SetExpandStringValue(name, value)
	} else {
		err = k.SetStringValue(name, value)
	}
	if err != nil {
		return fmt.Errorf("failed to restore %q: %w", name, err)
	}
	m.BroadcastChange()
	return nil
}

// isExpandString reports whether the existing value is REG_EXPAND_SZ.
// Any error (e.g. value does not exist yet) means "no".
func (m *Manager) isExpandString(name string, scope Scope) bool {
	k, err := m.openKey(scope)
	if err != nil {
		return false
	}
	defer k.Close()
	_, vt, err := k.GetValue(name, nil)
	return err == nil && vt == registry.EXPAND_SZ
}
