/*
 FILE: internal/cli/edit.go

 `pathman edit` — change an entry inside ANY ;-separated variable
 (PATH, PATHEXT, PYTHONPATH, ...) in place, keeping its position.

   pathman edit PATH Python38 Python312        substring replace (case-insens.)
   pathman edit PATH "C:\Python3*" "C:\Py"     glob: whole entry replaced
   pathman edit PATH "^C:\\Old(.*)" "D:\New$1" -R   regex with $1 groups
   pathman edit PATH --at 3 "C:\NewTool"       replace entry #3 (see `find`)
*/

package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cumulus13/pathman/internal/env"
	"github.com/cumulus13/pathman/internal/pattern"
	"github.com/cumulus13/pathman/internal/ui"
)

type entryChange struct {
	Index int // 1-based, position in the original value
	Old   string
	New   string
}

// applyEdit returns the edited entries and the list of changes.
//
//	at > 0       : replace entry number `at` with newVal (old is ignored)
//	regex        : regexp.ReplaceAllString (supports $1 groups)
//	glob pattern : whole entry replaced by newVal when it matches
//	plain text   : case-insensitive substring replaced by newVal
func applyEdit(parts []string, old, newVal string, useRegex bool, at int) ([]string, []entryChange, error) {
	out := make([]string, len(parts))
	copy(out, parts)
	var changes []entryChange

	if at > 0 {
		if at > len(parts) {
			return nil, nil, fmt.Errorf("--at %d is out of range (variable has %d entries)", at, len(parts))
		}
		if parts[at-1] == "" {
			return nil, nil, fmt.Errorf("entry #%d is empty", at)
		}
		out[at-1] = newVal
		return out, []entryChange{{at, parts[at-1], newVal}}, nil
	}

	var transform func(string) (string, bool)
	switch {
	case useRegex:
		p := old
		if !strings.HasPrefix(p, "(?") {
			p = "(?i)" + p
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid regex %q: %w", old, err)
		}
		transform = func(s string) (string, bool) {
			if !re.MatchString(s) {
				return s, false
			}
			return re.ReplaceAllString(s, newVal), true
		}
	case pattern.IsPattern(old):
		re, err := regexp.Compile(globToRegex(old))
		if err != nil {
			return nil, nil, fmt.Errorf("invalid glob %q: %w", old, err)
		}
		transform = func(s string) (string, bool) {
			if !re.MatchString(s) {
				return s, false
			}
			return newVal, true
		}
	default:
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(old))
		transform = func(s string) (string, bool) {
			if !re.MatchString(s) {
				return s, false
			}
			return re.ReplaceAllLiteralString(s, newVal), true
		}
	}

	for i, p := range parts {
		if p == "" {
			continue
		}
		if np, ok := transform(p); ok && np != p {
			out[i] = np
			changes = append(changes, entryChange{i + 1, p, np})
		}
	}
	return out, changes, nil
}

func editCmd() *cobra.Command {
	var scope string
	var useRegex, confirm, refresh bool
	var at int

	cmd := &cobra.Command{
		Use:     "edit <variable> <old|pattern> <new>",
		Aliases: []string{"replace", "change", "sub"},
		Short:   fmt.Sprintf("%s Edit an entry inside a ;-separated variable (PATH, PATHEXT, ...)", ui.IconGear),
		Long: `Change entries inside any ;-separated variable, in place (order is kept).

Matching of <old>:
  plain text   case-insensitive SUBSTRING is replaced by <new>
  glob         (* ? [abc]) the WHOLE entry is replaced by <new>
  --regex/-R   regexp replace, $1 $2 groups allowed in <new>
  --at N       replace entry number N (numbers shown by 'pathman find');
               usage: pathman edit VAR --at N <new>

When more than one entry would change, --confirm is required.
Use --dry-run to preview.

Examples:
  pathman edit PATH Python38 Python312
  pathman edit PATH "C:\Python3*" "C:\Python312" --confirm
  pathman edit PATH "^C:\\Old(.*)" "D:\New$1" -R
  pathman edit PATH --at 4 "C:\Tools\bin" -s system
  pathman edit PYTHONPATH "C:\lib" "D:\lib" --dry-run`,
		Args: func(cmd *cobra.Command, args []string) error {
			if at > 0 {
				return cobra.ExactArgs(2)(cmd, args)
			}
			return cobra.ExactArgs(3)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			sc := scopeFromString(scope)
			varName := args[0]

			var old, newVal string
			if at > 0 {
				newVal = args[1]
			} else {
				old, newVal = args[1], args[2]
			}

			v, err := m.Get(varName, sc)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}

			parts := strings.Split(v.Value, string(os.PathListSeparator))
			out, changes, err := applyEdit(parts, old, newVal, useRegex, at)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(err.Error()))
				return err
			}
			if len(changes) == 0 {
				fmt.Printf("%s No entry in %s matches %s\n", ui.IconWarning, ui.Highlight(varName),
					ui.Warning(pattern.Describe(old, useRegex)))
				return nil
			}

			if len(changes) > 1 && !confirm {
				fmt.Printf("%s %d entries in %s would change. Use --confirm to apply:\n",
					ui.IconWarning, len(changes), ui.Highlight(varName))
				for _, c := range changes {
					fmt.Printf("  #%-3d %s\n       %s %s\n", c.Index, ui.Dim(c.Old), ui.Dim("→"), ui.Path(c.New))
				}
				return fmt.Errorf("use --confirm to change %d entries", len(changes))
			}

			if dryRun {
				for _, c := range changes {
					fmt.Printf("%s %s #%d: %s %s %s\n", ui.IconInfo, ui.Info("Would change"), c.Index,
						ui.Dim(c.Old), ui.Dim("→"), ui.Path(c.New))
				}
				return nil
			}

			newValue := env.NormalizePathSeparator(strings.Join(out, string(os.PathListSeparator)))
			if refresh {
				err = m.SetAndRefresh(varName, newValue, sc)
			} else {
				err = m.Set(varName, newValue, sc)
			}
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			for _, c := range changes {
				fmt.Printf("%s %s #%d in %s (%s)\n     %s %s %s\n", ui.IconSuccess, "Changed", c.Index,
					ui.Highlight(varName), ui.Dim(sc.String()), ui.Dim(c.Old), ui.Dim("→"), ui.Path(c.New))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat <old> as regex ($1 groups allowed in <new>)")
	cmd.Flags().IntVar(&at, "at", 0, "Replace entry number N (1-based, as shown by 'find')")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm when more than one entry changes")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Broadcast WM_SETTINGCHANGE after writing")
	return cmd
}
