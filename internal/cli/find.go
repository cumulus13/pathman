/*
 FILE: internal/cli/find.go

 `pathman find` — search environment variables.

   pathman find <query>                 search everything: names, values and
                                        list entries (PATH-style, split on ;)
                                        in BOTH user and system scope
   pathman find <query> -e PATH         search only inside one variable
   pathman find <query> --in name       restrict to names | value | entry
   pathman find <query> -s system       restrict scope: user | system | both

 Query syntax:
   plain text   case-insensitive substring       (python  -> C:\Python312\Scripts)
   glob         * ? [abc], matches the whole text (C:\Python3*)
   --regex/-R   Go regexp, unanchored, case-insensitive by default
*/

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cumulus13/pathman/internal/env"
	"github.com/cumulus13/pathman/internal/pattern"
	"github.com/cumulus13/pathman/internal/ui"
)

// FindHit is a single match produced by `pathman find`.
type FindHit struct {
	Variable string `json:"variable"`
	Scope    string `json:"scope"`
	Kind     string `json:"kind"`            // name | value | entry
	Index    int    `json:"index,omitempty"` // 1-based position for kind=entry
	Match    string `json:"match"`           // the text that matched
	Exists   *bool  `json:"exists,omitempty"`
}

// finder compiles the query once and tests arbitrary text against it.
type finder struct {
	test func(string) bool
}

func newFinder(query string, useRegex bool) (*finder, error) {
	switch {
	case useRegex:
		p := query
		if !strings.HasPrefix(p, "(?") {
			p = "(?i)" + p
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", query, err)
		}
		return &finder{test: re.MatchString}, nil
	case pattern.IsPattern(query):
		// Own glob -> regex translation: filepath.Match treats \ as a path
		// separator on Windows and stops '*' at it, so "C:\\Go*" would never
		// match "C:\\Go\\bin". Here '*' crosses separators.
		re, err := regexp.Compile(globToRegex(query))
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", query, err)
		}
		return &finder{test: re.MatchString}, nil
	default:
		q := strings.ToLower(query)
		return &finder{test: func(s string) bool { return strings.Contains(strings.ToLower(s), q) }}, nil
	}
}

// globToRegex converts * ? [abc] [!abc] into an anchored, case-insensitive regexp.
func globToRegex(g string) string {
	var sb strings.Builder
	sb.WriteString("(?is)^")
	inClass := false
	prev := rune(0)
	for _, r := range g {
		if inClass && prev == '[' && r == '!' { // [!abc] -> [^abc]
			sb.WriteRune('^')
			prev = r
			continue
		}
		prev = r
		switch {
		case inClass:
			switch r {
			case ']':
				inClass = false
				sb.WriteRune(r)
			case '\\':
				sb.WriteString(`\\`)
			default:
				sb.WriteRune(r)
			}
		case r == '*':
			sb.WriteString(".*")
		case r == '?':
			sb.WriteString(".")
		case r == '[':
			inClass = true
			sb.WriteRune(r)
		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	if inClass { // unterminated '[' -> treat literally so the error is clear
		return "(?is)^" + regexp.QuoteMeta(g) + "$"
	}
	sb.WriteString("$")
	return sb.String()
}

// isListValue reports whether a value looks like a ;-separated list.
func isListValue(v string) bool { return strings.Contains(v, ";") }

func findInVariable(v env.Variable, f *finder, in string, checkExist bool) []FindHit {
	var hits []FindHit
	scope := v.Scope.String()

	if (in == "all" || in == "name") && f.test(v.Name) {
		hits = append(hits, FindHit{Variable: v.Name, Scope: scope, Kind: "name", Match: v.Name})
	}
	if in == "name" {
		return hits
	}

	if isListValue(v.Value) {
		if in == "all" || in == "entry" {
			for i, part := range strings.Split(v.Value, ";") {
				if part == "" || !f.test(part) {
					continue
				}
				h := FindHit{Variable: v.Name, Scope: scope, Kind: "entry", Index: i + 1, Match: part}
				if checkExist {
					ex := dirExists(part)
					h.Exists = &ex
				}
				hits = append(hits, h)
			}
		}
		if in == "value" && f.test(v.Value) {
			hits = append(hits, FindHit{Variable: v.Name, Scope: scope, Kind: "value", Match: v.Value})
		}
		return hits
	}

	if (in == "all" || in == "value" || in == "entry") && v.Value != "" && f.test(v.Value) {
		h := FindHit{Variable: v.Name, Scope: scope, Kind: "value", Match: v.Value}
		hits = append(hits, h)
	}
	return hits
}

func dirExists(p string) bool {
	expanded := os.ExpandEnv(p)
	info, err := os.Stat(expanded)
	return err == nil && info.IsDir()
}

func findCmd() *cobra.Command {
	var scope, onlyVar, in string
	var useRegex, checkExist, namesOnly bool

	cmd := &cobra.Command{
		Use:     "find <query>",
		Aliases: []string{"search", "grep", "fd", "locate"},
		Short:   fmt.Sprintf("%s Find text in environment variables (names, values, PATH entries)", ui.IconSearch),
		Long: `Search environment variables for a name, value or PATH-style entry.

By default searches EVERYTHING: variable names, values, and each entry of
;-separated lists (PATH, PATHEXT, ...) in both user and system scope.

  -e/--env NAME   search only inside ONE variable (e.g. PATH)
  --in WHAT       limit to: all (default) | name | value | entry
  -s/--scope      user | system | both (default: both)
  -R/--regex      query is a regular expression (case-insensitive)
  --exists        mark list entries whose directory is missing on disk
  -l/--list       print only the names of matching variables

Query: plain text = case-insensitive substring; with * ? [..] = glob
(matches the whole text); with --regex = unanchored regexp.

Examples:
  pathman find python                  # everywhere
  pathman find python -e PATH          # only inside PATH
  pathman find "C:\Program Files*" -s system
  pathman find "java" --in name        # only variable names
  pathman find "^C:\\Go" -R -e PATH --exists
  pathman find mingw -l                # just list variable names
  pathman find python -f json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			in = strings.ToLower(in)
			switch in {
			case "all", "name", "value", "entry":
			case "names":
				in = "name"
			case "values":
				in = "value"
			case "entries", "paths", "path":
				in = "entry"
			default:
				return fmt.Errorf("invalid --in %q (use all, name, value or entry)", in)
			}

			f, err := newFinder(query, useRegex)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(err.Error()))
				return err
			}

			m := env.NewManager(dryRun)

			var scopes []env.Scope
			switch strings.ToLower(scope) {
			case "user":
				scopes = []env.Scope{env.ScopeUser}
			case "system", "machine", "global":
				scopes = []env.Scope{env.ScopeSystem}
			case "both", "all", "":
				scopes = []env.Scope{env.ScopeUser, env.ScopeSystem}
			default:
				return fmt.Errorf("invalid --scope %q (use user, system or both)", scope)
			}

			var hits []FindHit
			scanned := 0

			if onlyVar != "" {
				// Single-variable search: look at entries/value of that variable only.
				// --in name makes no sense here (the name is already given).
				scopeIn := in
				if scopeIn == "all" || scopeIn == "name" {
					scopeIn = "entry"
				}
				found := false
				for _, sc := range scopes {
					v, gerr := m.Get(onlyVar, sc)
					if gerr != nil || v == nil {
						continue
					}
					found = true
					scanned++
					hits = append(hits, findInVariable(*v, f, scopeIn, checkExist)...)
				}
				if !found {
					msg := fmt.Sprintf("variable %q not found in %s scope", onlyVar, scope)
					fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(msg))
					return fmt.Errorf("%s", msg)
				}
			} else {
				for _, sc := range scopes {
					vars, lerr := m.List(sc)
					if lerr != nil {
						// system scope may be unreadable without admin; keep going
						fmt.Fprintf(os.Stderr, "%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("%s scope: %v", sc, lerr)))
						continue
					}
					for _, v := range vars {
						scanned++
						hits = append(hits, findInVariable(v, f, in, checkExist)...)
					}
				}
			}

			if namesOnly {
				hits = uniqueByVariable(hits)
			}
			return printFindHits(hits, query, scanned, namesOnly)
		},
	}

	cmd.Flags().StringVarP(&scope, "scope", "s", "both", "Scope: user, system, or both")
	cmd.Flags().StringVarP(&onlyVar, "env", "e", "", "Search only inside this variable (e.g. PATH)")
	cmd.Flags().StringVar(&in, "in", "all", "Where to look: all, name, value, entry")
	cmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat query as regex instead of text/glob")
	cmd.Flags().BoolVar(&checkExist, "exists", false, "Mark PATH-style entries whose directory does not exist")
	cmd.Flags().BoolVarP(&namesOnly, "list", "l", false, "Print only matching variable names")
	return cmd
}

func uniqueByVariable(hits []FindHit) []FindHit {
	seen := map[string]bool{}
	var out []FindHit
	for _, h := range hits {
		key := h.Scope + "\x00" + strings.ToLower(h.Variable)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, h)
	}
	return out
}

func printFindHits(hits []FindHit, query string, scanned int, namesOnly bool) error {
	switch strings.ToLower(format) {
	case "json":
		out := struct {
			Query   string    `json:"query"`
			Scanned int       `json:"scanned_variables"`
			Total   int       `json:"total"`
			Hits    []FindHit `json:"hits"`
		}{query, scanned, len(hits), hits}
		if out.Hits == nil {
			out.Hits = []FindHit{}
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	case "csv":
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{"Variable", "Scope", "Kind", "Index", "Match", "Exists"})
		for _, h := range hits {
			idx, ex := "", ""
			if h.Index > 0 {
				idx = fmt.Sprint(h.Index)
			}
			if h.Exists != nil {
				ex = fmt.Sprint(*h.Exists)
			}
			_ = w.Write([]string{h.Variable, h.Scope, h.Kind, idx, h.Match, ex})
		}
		w.Flush()
		return w.Error()
	}

	if len(hits) == 0 {
		fmt.Printf("%s No match for %s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("%q", query)),
			ui.Dim(fmt.Sprintf("(scanned %d variables)", scanned)))
		return nil
	}

	if namesOnly {
		for _, h := range hits {
			fmt.Printf("%s %s %s\n", ui.GetScopeIcon(strings.ToLower(h.Scope)), ui.Highlight(h.Variable), ui.Dim("("+h.Scope+")"))
		}
		fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("%d variables matched", len(hits))))
		return nil
	}

	// group consecutive hits by (scope, variable)
	lastKey := ""
	for _, h := range hits {
		key := h.Scope + "\x00" + h.Variable
		if key != lastKey {
			fmt.Printf("\n%s %s %s\n", ui.GetScopeIcon(strings.ToLower(h.Scope)), ui.Highlight(h.Variable), ui.Dim("("+h.Scope+")"))
			lastKey = key
		}
		switch h.Kind {
		case "name":
			fmt.Printf("  %s %s\n", ui.Dim("name   "), ui.KeyValue(h.Match))
		case "value":
			fmt.Printf("  %s %s\n", ui.Dim("value  "), ui.Path(h.Match))
		case "entry":
			mark := ""
			if h.Exists != nil {
				if *h.Exists {
					mark = " " + ui.IconSuccess
				} else {
					mark = " " + ui.IconBroken + " " + ui.Error("missing")
				}
			}
			fmt.Printf("  %s %s%s\n", ui.Dim(fmt.Sprintf("#%-5d", h.Index)), ui.Path(h.Match), mark)
		}
	}
	fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("%d match(es) in %d variables scanned", len(hits), scanned)))
	return nil
}
