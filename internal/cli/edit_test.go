package cli

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var sdk = []string{
	`C:\Windows`,
	`c:\SDK\lua`,
	`c:\SDK\luarocks`,
	`c:\SDK\lua\systree\bin`,
	``,
	`C:\Python38\`,
}

func idx(ch []entryChange) []int {
	var o []int
	for _, c := range ch {
		o = append(o, c.Index)
	}
	return o
}

// Regression: the reported bug. Plain text must NOT rewrite "luarocks".
func TestPlainTextIsWholeEntry(t *testing.T) {
	out, ch, err := applyEdit(sdk, `c:\sdk\lua`, `c:\SDK\lua\55`, false, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(idx(ch), []int{2}) {
		t.Fatalf("changed %v, want only #2", idx(ch))
	}
	if out[1] != `c:\SDK\lua\55` || out[2] != `c:\SDK\luarocks` || out[3] != `c:\SDK\lua\systree\bin` {
		t.Fatalf("collateral damage: %v", out)
	}
}

func TestTrailingSlashAndForwardSlashStillMatch(t *testing.T) {
	_, ch, _ := applyEdit(sdk, `c:/python38`, `D:\Py`, false, false, 0)
	if !reflect.DeepEqual(idx(ch), []int{6}) {
		t.Fatalf("got %v", idx(ch))
	}
}

func TestContainsIsOptIn(t *testing.T) {
	out, ch, _ := applyEdit(sdk, `c:\sdk\lua`, `X`, false, true, 0)
	if !reflect.DeepEqual(idx(ch), []int{2, 3, 4}) {
		t.Fatalf("got %v", idx(ch))
	}
	if out[2] != `Xrocks` {
		t.Fatalf("got %q", out[2])
	}
}

func TestGlobReplacesWholeEntry(t *testing.T) {
	out, ch, _ := applyEdit(sdk, `c:\SDK\lua*`, `Z`, false, false, 0)
	if !reflect.DeepEqual(idx(ch), []int{2, 3, 4}) || out[3] != "Z" {
		t.Fatalf("%v %v", idx(ch), out)
	}
}

func TestRegexGroups(t *testing.T) {
	out, ch, err := applyEdit(sdk, `^c:\\SDK\\lua\\(.*)$`, `D:\L\$1`, true, false, 0)
	if err != nil || !reflect.DeepEqual(idx(ch), []int{4}) || out[3] != `D:\L\systree\bin` {
		t.Fatalf("%v %v %v", err, idx(ch), out)
	}
	if _, _, err := applyEdit(sdk, `(`, `x`, true, false, 0); err == nil {
		t.Fatal("bad regex must error")
	}
}

func TestAtIndex(t *testing.T) {
	out, ch, err := applyEdit(sdk, "", `N`, false, false, 3)
	if err != nil || len(ch) != 1 || out[2] != "N" || out[1] != `c:\SDK\lua` {
		t.Fatalf("%v %v", err, out)
	}
	for _, bad := range []int{5, 99} { // 5 = empty entry, 99 = out of range
		if _, _, err := applyEdit(sdk, "", "N", false, false, bad); err == nil {
			t.Errorf("--at %d should fail", bad)
		}
	}
}

func TestEmptyEntriesPreservedNotMatched(t *testing.T) {
	out, ch, _ := applyEdit(sdk, ``, `N`, false, true, 0) // contains "" matches everything non-empty
	_ = ch
	if out[4] != "" {
		t.Fatal("empty entry must stay empty")
	}
}

func TestNoMatchChangesNothing(t *testing.T) {
	out, ch, err := applyEdit(sdk, `nothing`, `x`, false, false, 0)
	if err != nil || len(ch) != 0 || !reflect.DeepEqual(out, sdk) {
		t.Fatal("unexpected change")
	}
}

func TestGlobToRegex(t *testing.T) {
	cases := []struct {
		glob, s string
		want    bool
	}{
		{`C:\Go*`, `C:\Go\bin`, true}, // '*' crosses backslashes
		{`C:\Go*`, `D:\Go`, false},
		{`py?hon`, `Python`, true},
		{`C:\[GW]*`, `C:\Windows`, true},
		{`C:\[!G]*`, `C:\Go\bin`, false},
		{`C:\[!G]*`, `C:\Windows`, true},
		{`a.b*`, `axb`, false}, // '.' is literal
		{`[`, `[`, true},       // unterminated class is literal
	}
	for _, c := range cases {
		re := globToRegex(c.glob)
		got := regexpMust(t, re).MatchString(c.s)
		if got != c.want {
			t.Errorf("glob %q vs %q: got %v want %v (%s)", c.glob, c.s, got, c.want, strings.TrimSpace(re))
		}
	}
}

func regexpMust(t *testing.T, p string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	return re
}
