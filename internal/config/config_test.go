package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// isolate: no real config/home/env leaks into the test
func isolate(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	for _, k := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "APPDATA", "LOCALAPPDATA", EnvConfig, EnvBackupDir} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	return home
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestDefaults(t *testing.T) {
	isolate(t)
	s, err := Resolve(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !s.BackupEnabled || s.BackupKeep != DefaultKeep || s.ConfigFile != "" {
		t.Fatalf("bad defaults: %+v", s)
	}
	if !filepath.IsAbs(s.BackupDir) || !strings.Contains(s.BackupDir, "pathman") {
		t.Fatalf("bad default dir %q", s.BackupDir)
	}
}

// The whole point of the "fatal bug" concern: a config file in the CWD must
// never be picked up.
func TestCwdConfigIsIgnored(t *testing.T) {
	isolate(t)
	cwd := t.TempDir()
	write(t, filepath.Join(cwd, "pathman.json"), `{"backup":{"enabled":false}}`)
	write(t, filepath.Join(cwd, "config.json"), `{"backup":{"enabled":false}}`)
	chdir(t, cwd)
	s, err := Resolve(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if s.ConfigFile != "" || !s.BackupEnabled {
		t.Fatalf("cwd config was used: %+v", s)
	}
}

func TestResultIndependentOfCwd(t *testing.T) {
	home := isolate(t)
	cfg := filepath.Join(home, "c.json")
	write(t, cfg, `{"backup":{"dir":"bk","keep":7}}`)
	a, b := t.TempDir(), t.TempDir()
	chdir(t, a)
	s1, err := Resolve(Overrides{ConfigPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, b)
	s2, err := Resolve(Overrides{ConfigPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if s1.BackupDir != s2.BackupDir {
		t.Fatalf("dir depends on cwd: %q vs %q", s1.BackupDir, s2.BackupDir)
	}
	// relative dir is anchored at the config file's folder
	if want := filepath.Join(home, "bk"); s1.BackupDir != want {
		t.Fatalf("got %q want %q", s1.BackupDir, want)
	}
	if s1.BackupKeep != 7 {
		t.Fatal("keep not read")
	}
}

func TestDefaultLocationIsUsed(t *testing.T) {
	isolate(t)
	p, err := DefaultConfigPath()
	if err != nil {
		t.Skip(err)
	}
	write(t, p, `{"backup":{"keep":3}}`)
	s, err := Resolve(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if s.ConfigFile != p || s.BackupKeep != 3 {
		t.Fatalf("%+v (want file %s)", s, p)
	}
}

func TestExplicitMissingIsError(t *testing.T) {
	isolate(t)
	if _, err := Resolve(Overrides{ConfigPath: filepath.Join(t.TempDir(), "nope.json")}); err == nil {
		t.Fatal("expected error for missing --config")
	}
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "nope.json"))
	if _, err := Resolve(Overrides{}); err == nil {
		t.Fatal("expected error for missing $PATHMAN_CONFIG")
	}
}

func TestBrokenConfigIsError(t *testing.T) {
	home := isolate(t)
	for name, body := range map[string]string{
		"syntax":  `{"backup": {`,
		"unknown": `{"backup":{"backup_dir":"x"}}`,
		"badkeep": `{"backup":{"keep":-1}}`,
		"badtype": `{"backup":{"enabled":"yes"}}`,
	} {
		p := filepath.Join(home, name+".json")
		write(t, p, body)
		if _, err := Resolve(Overrides{ConfigPath: p}); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestBOMAndEmptyFile(t *testing.T) {
	home := isolate(t)
	p := filepath.Join(home, "bom.json")
	write(t, p, "\xef\xbb\xbf"+`{"backup":{"keep":5}}`)
	s, err := Resolve(Overrides{ConfigPath: p})
	if err != nil || s.BackupKeep != 5 {
		t.Fatalf("bom: %v %+v", err, s)
	}
	e := filepath.Join(home, "empty.json")
	write(t, e, "  \n")
	if _, err := Resolve(Overrides{ConfigPath: e}); err != nil {
		t.Fatalf("empty file should be fine: %v", err)
	}
}

func TestDirPrecedence(t *testing.T) {
	home := isolate(t)
	abs := func(n string) string { return filepath.Join(home, n) }
	cfg := abs("c.json")
	write(t, cfg, `{"backup":{"dir":"`+strings.ReplaceAll(abs("fromcfg"), `\`, `\\`)+`"}}`)

	s, _ := Resolve(Overrides{ConfigPath: cfg})
	if s.BackupDir != abs("fromcfg") || s.DirSource != "config file" {
		t.Fatalf("config: %+v", s)
	}
	t.Setenv(EnvBackupDir, abs("fromenv"))
	s, _ = Resolve(Overrides{ConfigPath: cfg})
	if s.BackupDir != abs("fromenv") {
		t.Fatalf("env should beat config: %+v", s)
	}
	s, _ = Resolve(Overrides{ConfigPath: cfg, BackupDir: abs("fromcli")})
	if s.BackupDir != abs("fromcli") || s.DirSource != "--backup-dir" {
		t.Fatalf("cli should win: %+v", s)
	}
}

func TestNoBackupFlagAndConfigDisable(t *testing.T) {
	home := isolate(t)
	s, _ := Resolve(Overrides{NoBackup: true})
	if s.BackupEnabled {
		t.Fatal("--no-backup ignored")
	}
	p := filepath.Join(home, "off.json")
	write(t, p, `{"backup":{"enabled":false}}`)
	s, _ = Resolve(Overrides{ConfigPath: p})
	if s.BackupEnabled {
		t.Fatal("config enabled:false ignored")
	}
}

func TestVarExpansionInDir(t *testing.T) {
	home := isolate(t)
	t.Setenv("PM_TEST_BASE", home)
	p := filepath.Join(home, "v.json")
	write(t, p, `{"backup":{"dir":"%PM_TEST_BASE%/viaWin"}}`)
	s, err := Resolve(Overrides{ConfigPath: p})
	if err != nil || s.BackupDir != filepath.Join(home, "viaWin") {
		t.Fatalf("%v %+v", err, s)
	}
	write(t, p, `{"backup":{"dir":"%PM_DOES_NOT_EXIST%/x"}}`)
	if _, err := Resolve(Overrides{ConfigPath: p}); err == nil && runtime.GOOS != "plan9" {
		t.Fatal("unresolved %VAR% must be an error, not a literal folder name")
	}
}

func TestTemplateRoundTrips(t *testing.T) {
	home := isolate(t)
	tpl, err := Template()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(home, "t.json")
	write(t, p, tpl)
	if _, err := Resolve(Overrides{ConfigPath: p}); err != nil {
		t.Fatalf("template does not load: %v\n%s", err, tpl)
	}
}
