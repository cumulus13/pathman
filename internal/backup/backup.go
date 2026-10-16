// Package backup stores point-in-time snapshots of an environment scope.
//
// A snapshot is written BEFORE any registry change. The write is atomic
// (temp file -> fsync -> rename) and verified by reading it back; callers must
// abort the change if Write returns an error.
package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	FormatVersion = 1
	Tool          = "pathman"
)

// Value is one registry value. Type is "REG_SZ", "REG_EXPAND_SZ" or "REG_<n>".
// String types use Value; every other type is kept verbatim in Raw (base64).
type Value struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
	Raw   []byte `json:"raw,omitempty"` // encoded as base64 by encoding/json
}

// IsString reports whether the value can be restored by pathman.
func (v Value) IsString() bool { return v.Type == "REG_SZ" || v.Type == "REG_EXPAND_SZ" }

// Snapshot is the on-disk format.
type Snapshot struct {
	Version int      `json:"version"`
	Tool    string   `json:"tool"`
	Created string   `json:"created"` // RFC3339 UTC
	Scope   string   `json:"scope"`   // "user" | "system"
	Action  string   `json:"action"`  // e.g. "pathman edit"
	Args    []string `json:"args,omitempty"`
	User    string   `json:"user,omitempty"`
	Host    string   `json:"host,omitempty"`
	Values  []Value  `json:"values"`
}

// Entry is a snapshot file found on disk.
type Entry struct {
	Path     string
	Snapshot *Snapshot
}

// only files named like this are ever listed, restored or pruned, so pointing
// the backup dir at a folder with other JSON files can never delete them.
//
//	<UTC yyyymmddThhmmss.mmmZ>-<seq 000-999>_<scope>.json
//
// The fixed-width layout makes lexical order identical to chronological order.
var nameRe = regexp.MustCompile(`^\d{8}T\d{6}\.\d{3}Z-\d{3}_(user|system)\.json$`)

// New builds a snapshot header.
func New(scope, action string, args []string, values []Value) *Snapshot {
	host, _ := os.Hostname()
	user := os.Getenv("USERNAME")
	if user == "" {
		user = os.Getenv("USER")
	}
	return &Snapshot{
		Version: FormatVersion,
		Tool:    Tool,
		Created: time.Now().UTC().Format(time.RFC3339),
		Scope:   strings.ToLower(scope),
		Action:  action,
		Args:    args,
		User:    user,
		Host:    host,
		Values:  values,
	}
}

// Write stores s in dir and returns the file path. dir is created if needed.
func Write(dir string, s *Snapshot) (string, error) {
	if dir == "" {
		return "", errors.New("backup directory is empty")
	}
	if st, err := os.Stat(dir); err == nil && !st.IsDir() {
		return "", fmt.Errorf("backup path %s exists and is not a directory", dir)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create backup directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, ".pathman-tmp-*")
	if err != nil {
		return "", fmt.Errorf("cannot write to backup directory %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", err
	}

	stamp := time.Now().UTC().Format("20060102T150405.000Z")
	final := ""
	for i := 0; i < 1000; i++ {
		prefix := fmt.Sprintf("%s-%03d_", stamp, i)
		// never overwrite: the sequence number must be free for EVERY scope,
		// so names stay strictly ordered in time
		if !prefixTaken(dir, prefix) {
			final = filepath.Join(dir, prefix+s.Scope+".json")
			break
		}
	}
	if final == "" {
		cleanup()
		return "", errors.New("could not pick a unique backup file name")
	}
	if err := os.Rename(tmpName, final); err != nil {
		cleanup()
		return "", fmt.Errorf("cannot finalize backup: %w", err)
	}

	// verify by reading back
	got, err := Read(final)
	if err != nil {
		_ = os.Remove(final)
		return "", fmt.Errorf("backup verification failed: %w", err)
	}
	if err := equal(s, got); err != nil {
		_ = os.Remove(final)
		return "", fmt.Errorf("backup verification failed: %w", err)
	}
	return final, nil
}

func prefixTaken(dir, prefix string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return true // be safe: treat as taken
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), prefix) {
			return true
		}
	}
	return false
}

func equal(a, b *Snapshot) error {
	if len(a.Values) != len(b.Values) {
		return fmt.Errorf("value count mismatch (%d vs %d)", len(a.Values), len(b.Values))
	}
	for i := range a.Values {
		x, y := a.Values[i], b.Values[i]
		if x.Name != y.Name || x.Type != y.Type || x.Value != y.Value || string(x.Raw) != string(y.Raw) {
			return fmt.Errorf("value %q differs after write", x.Name)
		}
	}
	return nil
}

// Read loads and validates one snapshot file.
func Read(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if s.Tool != Tool || s.Version != FormatVersion {
		return nil, fmt.Errorf("%s: not a pathman v%d snapshot", path, FormatVersion)
	}
	if s.Scope != "user" && s.Scope != "system" {
		return nil, fmt.Errorf("%s: invalid scope %q", path, s.Scope)
	}
	return &s, nil
}

// List returns snapshots newest first. scope "" = all. Unreadable files are skipped.
func List(dir, scope string) ([]Entry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && nameRe.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	var out []Entry
	for _, n := range names {
		p := filepath.Join(dir, n)
		s, err := Read(p)
		if err != nil {
			continue
		}
		if scope != "" && s.Scope != strings.ToLower(scope) {
			continue
		}
		out = append(out, Entry{Path: p, Snapshot: s})
	}
	return out, nil
}

// Resolve turns "latest", a file name inside dir, or a path into a file path.
func Resolve(dir, ref, scope string) (string, error) {
	if strings.EqualFold(ref, "latest") {
		es, err := List(dir, scope)
		if err != nil {
			return "", err
		}
		if len(es) == 0 {
			return "", fmt.Errorf("no backups found in %s", dir)
		}
		return es[0].Path, nil
	}
	if _, err := os.Stat(ref); err == nil {
		return filepath.Abs(ref)
	}
	p := filepath.Join(dir, ref)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("backup %q not found (looked in %s)", ref, dir)
}

// Prune keeps the newest `keep` snapshots (keep <= 0 keeps all) and returns
// how many were removed. Only files matching the snapshot naming scheme are
// ever touched.
func Prune(dir string, keep int) (int, error) {
	if keep <= 0 {
		return 0, nil
	}
	es, err := List(dir, "")
	if err != nil {
		return 0, err
	}
	removed := 0
	for i := keep; i < len(es); i++ {
		if err := os.Remove(es[i].Path); err == nil {
			removed++
		}
	}
	return removed, nil
}
