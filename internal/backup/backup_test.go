package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sample(scope string) *Snapshot {
	return New(scope, "pathman test", []string{"edit", "PATH"}, []Value{
		{Name: "PATH", Type: "REG_EXPAND_SZ", Value: `%SystemRoot%\system32;C:\Go\bin`},
		{Name: "JAVA_HOME", Type: "REG_SZ", Value: `C:\jdk`},
		{Name: "BLOB", Type: "REG_3", Raw: []byte{0, 1, 2, 255}},
	})
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "backups") // must be created
	s := sample("user")
	p, err := Write(dir, s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := equal(s, got); err != nil {
		t.Fatal(err)
	}
	if got.Values[0].Type != "REG_EXPAND_SZ" || got.Values[0].Value != s.Values[0].Value {
		t.Fatal("type/value (%SystemRoot% reference) not preserved")
	}
	// no temp files left behind
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		if strings.HasPrefix(e.Name(), ".pathman-tmp") {
			t.Fatalf("temp file left: %s", e.Name())
		}
	}
}

func TestNeverOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	seen := map[string]bool{}
	for i := 0; i < 25; i++ { // many writes inside the same millisecond
		p, err := Write(dir, sample("user"))
		if err != nil {
			t.Fatal(err)
		}
		if seen[p] {
			t.Fatalf("overwrote %s", p)
		}
		seen[p] = true
	}
	es, _ := List(dir, "")
	if len(es) != 25 {
		t.Fatalf("expected 25 backups, got %d", len(es))
	}
}

func TestDirIsFileFails(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile")
	os.WriteFile(f, []byte("x"), 0o600)
	if _, err := Write(f, sample("user")); err == nil {
		t.Fatal("expected error when backup dir is a file")
	}
	if _, err := Write("", sample("user")); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestListFiltersAndOrders(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.json"), []byte(`{"hello":1}`), 0o600)
	os.WriteFile(filepath.Join(dir, "20260101T000000.000Z-000_user.json"), []byte(`garbage`), 0o600)
	a, _ := Write(dir, sample("user"))
	b, _ := Write(dir, sample("system"))
	c, _ := Write(dir, sample("user"))

	all, _ := List(dir, "")
	if len(all) != 3 || all[0].Path != c || all[2].Path != a {
		t.Fatalf("order wrong: %v", all)
	}
	sys, _ := List(dir, "system")
	if len(sys) != 1 || sys[0].Path != b {
		t.Fatal("scope filter wrong")
	}
	if es, err := List(filepath.Join(dir, "missing"), ""); err != nil || es != nil {
		t.Fatal("missing dir should be empty, not an error")
	}
}

func TestPruneKeepsNewestAndNeverTouchesForeignFiles(t *testing.T) {
	dir := t.TempDir()
	foreign := []string{"notes.json", "important.json", "20260101T000000.000Z-000_user.json"} // last = bad content
	for _, f := range foreign {
		os.WriteFile(filepath.Join(dir, f), []byte(`{"keep":"me"}`), 0o600)
	}
	var paths []string
	for i := 0; i < 6; i++ {
		p, err := Write(dir, sample("user"))
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	n, err := Prune(dir, 2)
	if err != nil || n != 4 {
		t.Fatalf("removed %d, err %v", n, err)
	}
	for i, p := range paths {
		_, err := os.Stat(p)
		if i >= 4 && err != nil {
			t.Fatalf("newest %s was removed", p)
		}
		if i < 4 && err == nil {
			t.Fatalf("old %s kept", p)
		}
	}
	for _, f := range foreign {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("foreign file %s was deleted!", f)
		}
	}
	if n, _ := Prune(dir, 0); n != 0 {
		t.Fatal("keep=0 must keep everything")
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	if _, err := Resolve(dir, "latest", ""); err == nil {
		t.Fatal("latest with no backups must fail")
	}
	a, _ := Write(dir, sample("user"))
	b, _ := Write(dir, sample("system"))
	if p, _ := Resolve(dir, "latest", ""); p != b {
		t.Fatal("latest wrong")
	}
	if p, _ := Resolve(dir, "latest", "user"); p != a {
		t.Fatal("latest per scope wrong")
	}
	if p, err := Resolve(dir, filepath.Base(a), ""); err != nil || p != a {
		t.Fatal("by name failed")
	}
	if _, err := Resolve(dir, "nope.json", ""); err == nil {
		t.Fatal("expected not found")
	}
}

func TestReadRejectsForeignAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"a.json": `{"tool":"other","version":1,"scope":"user"}`,
		"b.json": `{"tool":"pathman","version":99,"scope":"user"}`,
		"c.json": `{"tool":"pathman","version":1,"scope":"galaxy"}`,
		"d.json": `{not json`,
	} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(body), 0o600)
		if _, err := Read(p); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
