// Package config loads pathman settings.
//
// Design rules (safety first):
//   - The config file is NEVER searched for in the current directory. Where
//     pathman is run from must not change what it does.
//   - A broken or unreadable config is a hard error, never silently ignored
//     (a typo must not quietly disable backups or move them elsewhere).
//   - Relative paths inside the config file are relative to the config file's
//     own directory, not the current directory.
//
// Config file lookup order (first hit wins):
//  1. --config <file>            (must exist)
//  2. $PATHMAN_CONFIG            (must exist)
//  3. <dir of pathman.exe>/pathman.json   (portable installs)
//  4. <user config dir>/pathman/config.json
//     Windows: %AppData%\pathman\config.json
//  5. no file -> built-in defaults
//
// Backup directory precedence: --backup-dir > $PATHMAN_BACKUP_DIR >
// config "backup.dir" > OS default.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	EnvConfig    = "PATHMAN_CONFIG"
	EnvBackupDir = "PATHMAN_BACKUP_DIR"

	DefaultKeep = 100
)

// File is the on-disk JSON structure.
type File struct {
	Backup BackupFile `json:"backup"`
}

// BackupFile holds the backup section. Pointers distinguish "unset" from zero.
type BackupFile struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Dir     string `json:"dir,omitempty"`
	Keep    *int   `json:"keep,omitempty"` // 0 = keep everything
}

// Overrides come from the command line.
type Overrides struct {
	ConfigPath string // --config
	BackupDir  string // --backup-dir
	NoBackup   bool   // --no-backup
}

// Settings is the fully resolved configuration.
type Settings struct {
	ConfigFile    string // "" when running on defaults
	BackupEnabled bool
	BackupDir     string
	BackupKeep    int

	// where each value came from (for `pathman config show`)
	EnabledSource string
	DirSource     string
	KeepSource    string
}

// Resolve discovers and loads configuration and applies overrides.
func Resolve(o Overrides) (*Settings, error) {
	path, err := discover(o.ConfigPath)
	if err != nil {
		return nil, err
	}

	var f File
	if path != "" {
		f, err = readFile(path)
		if err != nil {
			return nil, err
		}
	}

	s := &Settings{
		ConfigFile:    path,
		BackupEnabled: true,
		BackupKeep:    DefaultKeep,
		EnabledSource: "default",
		DirSource:     "default",
		KeepSource:    "default",
	}

	if f.Backup.Enabled != nil {
		s.BackupEnabled = *f.Backup.Enabled
		s.EnabledSource = "config file"
	}
	if f.Backup.Keep != nil {
		if *f.Backup.Keep < 0 {
			return nil, fmt.Errorf("%s: backup.keep must be >= 0", path)
		}
		s.BackupKeep = *f.Backup.Keep
		s.KeepSource = "config file"
	}
	if o.NoBackup {
		s.BackupEnabled = false
		s.EnabledSource = "--no-backup"
	}

	// backup dir: CLI > env > config > default
	switch {
	case o.BackupDir != "":
		d, err := resolveDir(o.BackupDir, "")
		if err != nil {
			return nil, fmt.Errorf("--backup-dir: %w", err)
		}
		s.BackupDir, s.DirSource = d, "--backup-dir"
	case os.Getenv(EnvBackupDir) != "":
		d, err := resolveDir(os.Getenv(EnvBackupDir), "")
		if err != nil {
			return nil, fmt.Errorf("$%s: %w", EnvBackupDir, err)
		}
		s.BackupDir, s.DirSource = d, "$"+EnvBackupDir
	case f.Backup.Dir != "":
		d, err := resolveDir(f.Backup.Dir, filepath.Dir(path))
		if err != nil {
			return nil, fmt.Errorf("%s: backup.dir: %w", path, err)
		}
		s.BackupDir, s.DirSource = d, "config file"
	default:
		d, err := DefaultBackupDir()
		if err != nil {
			return nil, err
		}
		s.BackupDir = d
	}
	return s, nil
}

// discover returns the config file to use ("" = none).
func discover(flagPath string) (string, error) {
	if flagPath != "" {
		p, err := filepath.Abs(expandVars(flagPath))
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("config file %s: %w", p, err)
		}
		return p, nil
	}
	if e := os.Getenv(EnvConfig); e != "" {
		p, err := filepath.Abs(expandVars(e))
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("$%s=%s: %w", EnvConfig, p, err)
		}
		return p, nil
	}
	for _, c := range candidates() {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", nil
}

// candidates lists the implicit config locations (never the cwd).
func candidates() []string {
	var out []string
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		out = append(out, filepath.Join(filepath.Dir(exe), "pathman.json"))
	}
	if p, err := DefaultConfigPath(); err == nil {
		out = append(out, p)
	}
	return out
}

// DefaultConfigPath is where `pathman config init` writes the file.
func DefaultConfigPath() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user config dir: %w", err)
	}
	return filepath.Join(d, "pathman", "config.json"), nil
}

// DefaultBackupDir returns the per-OS default backup location.
//
//	Windows: %LOCALAPPDATA%\pathman\backups
//	macOS:   ~/Library/Application Support/pathman/backups
//	others:  $XDG_STATE_HOME/pathman/backups (~/.local/state/pathman/backups)
func DefaultBackupDir() (string, error) {
	home, herr := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "pathman", "backups"), nil
		}
		if herr == nil {
			return filepath.Join(home, "AppData", "Local", "pathman", "backups"), nil
		}
	case "darwin":
		if herr == nil {
			return filepath.Join(home, "Library", "Application Support", "pathman", "backups"), nil
		}
	default:
		if d := os.Getenv("XDG_STATE_HOME"); d != "" && filepath.IsAbs(d) {
			return filepath.Join(d, "pathman", "backups"), nil
		}
		if herr == nil {
			return filepath.Join(home, ".local", "state", "pathman", "backups"), nil
		}
	}
	return "", errors.New("cannot determine a default backup directory; set one with --backup-dir or in the config file")
}

func readFile(path string) (File, error) {
	var f File
	data, err := os.ReadFile(path)
	if err != nil {
		return f, fmt.Errorf("cannot read config %s: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM (Notepad)
	if len(bytes.TrimSpace(data)) == 0 {
		return f, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields() // catch typos like "backup_dir"
	if err := dec.Decode(&f); err != nil {
		return f, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return f, nil
}

var winVar = regexp.MustCompile(`%([^%\s]+)%`)

// expandVars expands %VAR%, $VAR, ${VAR} and a leading ~.
func expandVars(p string) string {
	p = winVar.ReplaceAllStringFunc(p, func(m string) string {
		if v, ok := os.LookupEnv(m[1 : len(m)-1]); ok {
			return v
		}
		return m
	})
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	return p
}

// resolveDir expands variables and makes the path absolute.
// base == "" means "relative to the current directory" (explicit CLI input);
// otherwise relative paths are anchored at base (the config file's directory).
func resolveDir(raw, base string) (string, error) {
	p := strings.TrimSpace(expandVars(raw))
	if p == "" {
		return "", errors.New("empty directory")
	}
	if strings.Contains(p, "%") && winVar.MatchString(p) {
		return "", fmt.Errorf("unresolved variable in %q", raw)
	}
	if !filepath.IsAbs(p) {
		if base != "" {
			p = filepath.Join(base, p)
		} else {
			abs, err := filepath.Abs(p)
			if err != nil {
				return "", err
			}
			p = abs
		}
	}
	return filepath.Clean(p), nil
}

// Template returns a ready-to-edit config file for `config init`.
func Template() (string, error) {
	d, err := DefaultBackupDir()
	if err != nil {
		d = ""
	}
	f := File{Backup: BackupFile{Dir: d}}
	on := true
	keep := DefaultKeep
	f.Backup.Enabled, f.Backup.Keep = &on, &keep
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
