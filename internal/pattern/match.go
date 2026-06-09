/*
 FILE: internal/pattern/match.go

 Provides glob and regex pattern matching used by CLI commands.
 Glob supports: * (any chars), ? (one char), [abc] (char class) — case-insensitive on Windows.
 Regex uses Go's regexp package — case-insensitive by default unless (?-i) prefix is given.
*/

package pattern

import (
	"path/filepath"
	"regexp"
	"strings"
)

// IsPattern returns true if s contains any glob metacharacter (* ? [ ]).
// Used to auto-detect whether an arg is a literal or a pattern.
func IsPattern(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// Match tests value against pattern.
// If useRegex is true: pattern is a regular expression (case-insensitive by default).
// If useRegex is false: pattern is a shell glob (* ? [abc]), case-insensitive.
// An empty pattern matches everything.
func Match(pat, value string, useRegex bool) (bool, error) {
	if pat == "" {
		return true, nil
	}
	if useRegex {
		return matchRegex(pat, value)
	}
	return matchGlob(pat, value)
}

// MatchMust is like Match but panics on bad regex (use only when pattern is pre-validated).
func MatchMust(pat, value string, useRegex bool) bool {
	ok, err := Match(pat, value, useRegex)
	if err != nil {
		return false
	}
	return ok
}

// matchGlob does case-insensitive shell glob matching.
// Uses filepath.Match which supports * ? [abc] — we lowercase both sides first.
func matchGlob(pat, value string) (bool, error) {
	ok, err := filepath.Match(strings.ToLower(pat), strings.ToLower(value))
	if err != nil {
		return false, err
	}
	return ok, nil
}

// matchRegex does case-insensitive regex matching unless the pattern starts with (?-i).
func matchRegex(pat, value string) (bool, error) {
	// Prepend (?i) for case-insensitive unless the caller explicitly opted out
	if !strings.HasPrefix(pat, "(?") {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return false, err
	}
	return re.MatchString(value), nil
}

// FilterStrings returns only the elements of values that match pat.
func FilterStrings(pat string, values []string, useRegex bool) ([]string, error) {
	if pat == "" {
		return values, nil
	}
	var out []string
	for _, v := range values {
		ok, err := Match(pat, v, useRegex)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// Describe returns a human-readable label for what kind of pattern pat is.
func Describe(pat string, useRegex bool) string {
	if pat == "" {
		return ""
	}
	if useRegex {
		return "regex:" + pat
	}
	return "glob:" + pat
}
