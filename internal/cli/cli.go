/*
 FILE: internal/cli/cli.go  (v3 — wildcard/pattern support)

 Changes from v2:
   - New internal/pattern package (match.go) provides glob + regex matching.
   - get:          accepts glob/regex pattern -> shows all matching variables.
   - delete:       accepts glob/regex pattern -> deletes all matching (--confirm required for multi-match).
   - list:         --filter/-F glob/regex on variable names and/or values.
   - path list:    --filter/-F glob/regex on path entries.
   - path remove:  accepts glob/regex pattern -> removes all matching PATH entries.
   - remove-value: value arg accepts glob/regex pattern.
   - parse:        accepts glob/regex pattern -> formats all matching variables.
   - All pattern commands accept --regex/-R flag for regex mode (default: glob).
   - Auto-detect: if arg contains * ? [ it is treated as a glob automatically.
*/

package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/cumulus13/pathman/internal/env"
	"github.com/cumulus13/pathman/internal/pattern"
	"github.com/cumulus13/pathman/internal/ui"
)

var (
	manager *env.Manager
	rootCmd *cobra.Command
	dryRun  bool
	noColor bool
	format  string
)

func Execute() error {
	return rootCmd.Execute()
}

func copyToClipboard(text string) error {
	cmd := exec.Command("clip")
	cmd.Stdin = bytes.NewBufferString(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard copy failed: %w", err)
	}
	return nil
}

func init() {
	manager = env.NewManager(false)
	rootCmd = &cobra.Command{
		Use:   "pathman",
		Short: "🗺️  Professional Windows Environment Variable Manager",
		Long: `🌍 PathMan - Environment Variable Manager

A modern, professional Windows environment variable manager.
Manage PATH and other environment variables with ease.

Features:
  • Add, remove, and list PATH entries
  • Set, get, and delete environment variables
  • Append to variables with pre/post position
  • User and System scope management
  • Wildcard/glob and regex pattern matching
  • Duplicate detection and cleanup
  • Non-existent path validation
  • JSON, YAML, CSV, Table output formats
  • Dry-run mode for safe testing
  • Clipboard copy with -c/--clipboard
  • Instant terminal refresh with -r/--refresh
  • Temporary (current session only) with --temp

Pattern syntax (default: glob, case-insensitive):
  *          any sequence of characters
  ?          any single character
  [abc]      character class
  --regex    switch to full regex (Go regexp, case-insensitive by default)

Examples:
  pathman get "JAVA*"
  pathman get "*HOME*" -s system
  pathman delete "TEMP_*" --confirm
  pathman list --filter "*PATH*"
  pathman path list --filter "C:\Python*"
  pathman path remove "C:\Python3[78]*" --regex`,
		Version: "1.0.0",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				color.NoColor = true
			}
		},
	}
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "text", "Output format: text, json, yaml, csv, table")
	rootCmd.AddCommand(
		getCmd(), setCmd(), appendCmd(), deleteCmd(),
		pathCmd(), cleanCmd(), parseCmd(), removeValueCmd(),
		infoCmd(), listCmd(), refreshCmd(),
	)
}

func scopeFromString(s string) env.Scope {
	switch strings.ToLower(s) {
	case "system", "machine", "global":
		return env.ScopeSystem
	default:
		return env.ScopeUser
	}
}

func positionFromString(s string) env.AppendPosition {
	switch strings.ToLower(s) {
	case "pre", "front", "beginning", "start":
		return env.AppendPre
	default:
		return env.AppendPost
	}
}

// matchVars returns all variables in scope whose names match pat.
// If pat has no glob chars and useRegex is false, it tries exact match first,
// then falls back to a contains match so "PATH" still finds "PATH" directly.
func matchVars(m *env.Manager, pat string, scope env.Scope, useRegex bool) ([]env.Variable, error) {
	if !pattern.IsPattern(pat) && !useRegex {
		// Plain literal: get exactly that variable
		v, err := m.Get(pat, scope)
		if err != nil {
			return nil, err
		}
		return []env.Variable{*v}, nil
	}
	// Pattern: scan all variables in scope
	vars, err := m.List(scope)
	if err != nil {
		return nil, err
	}
	var matched []env.Variable
	for _, v := range vars {
		ok, err := pattern.Match(pat, v.Name, useRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pat, err)
		}
		if ok {
			matched = append(matched, v)
		}
	}
	return matched, nil
}

// matchBothScopes runs matchVars for both user and system scopes.
func matchBothScopes(m *env.Manager, pat string, useRegex bool) ([]env.Variable, error) {
	user, err := matchVars(m, pat, env.ScopeUser, useRegex)
	if err != nil {
		return nil, err
	}
	sys, err := matchVars(m, pat, env.ScopeSystem, useRegex)
	if err != nil {
		return nil, err
	}
	return append(user, sys...), nil
}

// ─── get ────────────────────────────────────────────────────────────────────

func getCmd() *cobra.Command {
	var scope string
	var clipboard bool
	var useRegex bool

	cmd := &cobra.Command{
		Use:     "get [variable|pattern]",
		Aliases: []string{"g", "show", "value"},
		Short:   fmt.Sprintf("%s Get environment variable value", ui.IconSearch),
		Long: `Retrieve and display the value of an environment variable.
Accepts glob patterns (* ? [abc]) or --regex for full regex.
If the pattern matches multiple variables, all are shown.

Examples:
  pathman get PATH
  pathman get "JAVA*"
  pathman get "*HOME*" -s system
  pathman get "^PYTHON" --regex`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			pat := args[0]

			// Determine scope(s) to search
			var vars []env.Variable
			var err error
			if strings.ToLower(scope) == "both" {
				vars, err = matchBothScopes(m, pat, useRegex)
			} else {
				vars, err = matchVars(m, pat, scopeFromString(scope), useRegex)
			}
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(err.Error()))
				return err
			}
			if len(vars) == 0 {
				fmt.Printf("%s No variables match %s\n", ui.IconWarning, ui.Warning(pattern.Describe(pat, useRegex)))
				return nil
			}

			if clipboard && len(vars) == 1 {
				if cerr := copyToClipboard(vars[0].Value); cerr != nil {
					fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
				} else {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Value copied to clipboard"))
				}
			}

			if format != "text" {
				output, _ := env.FormatVariables(vars, format)
				fmt.Print(output)
				return nil
			}
			for _, v := range vars {
				fmt.Printf("\n%s %s (%s)\n  %s\n",
					ui.GetScopeIcon(v.Scope.String()), ui.Highlight(v.Name),
					ui.Dim(v.Scope.String()), ui.Path(v.Value))
			}
			if len(vars) > 1 {
				fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("%d variables matched", len(vars))))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user, system, or both")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy value to clipboard (single match only)")
	cmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat pattern as regex instead of glob")
	return cmd
}

// ─── set ────────────────────────────────────────────────────────────────────

func setCmd() *cobra.Command {
	var scope string
	var refresh bool
	var clipboard bool

	cmd := &cobra.Command{
		Use:     "set [variable] [value]",
		Aliases: []string{"s"},
		Short:   fmt.Sprintf("%s Set environment variable (replaces entire value)", ui.IconSave),
		Long: `Create or REPLACE an environment variable with the specified value.
Pattern matching is intentionally NOT supported for set — mass-overwrite is too dangerous.
  -s/--scope     user (default) or system
  -r/--refresh   broadcast WM_SETTINGCHANGE to GUI apps
  -c/--clipboard copy new value to clipboard`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)
			varName := args[0]
			value := strings.Join(args[1:], " ")

			if dryRun {
				fmt.Printf("%s %s %s=%s (scope: %s)\n",
					ui.IconInfo, ui.Info("Would set"),
					ui.Highlight(varName), ui.Path(value), ui.Dim(scopeType.String()))
				return nil
			}

			if refresh {
				if err := m.SetAndRefresh(varName, value, scopeType); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
			} else {
				if err := m.Set(varName, value, scopeType); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
			}

			if clipboard {
				if cerr := copyToClipboard(value); cerr != nil {
					fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
				} else {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Value copied to clipboard"))
				}
			}

			fmt.Printf("%s %s %s=%s (%s %s)",
				ui.IconSuccess, ui.Success("Successfully set"),
				ui.Highlight(varName), ui.Path(value),
				ui.GetScopeIcon(scope), ui.Dim(scopeType.String()))
			if refresh {
				fmt.Printf(" %s", ui.Info("[WM_SETTINGCHANGE broadcast]"))
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Broadcast WM_SETTINGCHANGE to GUI apps after registry write")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy new value to clipboard")
	return cmd
}

// ─── append ─────────────────────────────────────────────────────────────────

func appendCmd() *cobra.Command {
	var scope string
	var position string
	var refresh bool
	var clipboard bool
	var temp bool
	var shell string

	cmd := &cobra.Command{
		Use:     "append [variable] [value]",
		Aliases: []string{"app", "concat", "add-to", "a", "add"},
		Short:   fmt.Sprintf("%s Append value to variable", ui.IconPlus),
		Long: `Append value to existing variable at beginning (pre) or end (post).
Duplicates are detected and prevented.
Pattern matching is NOT applied here — the variable name must be exact.

  --temp          Print shell command to update CURRENT terminal only (no registry write).
                  Pipe output through eval (cmd) or Invoke-Expression (PowerShell).
  --shell         Shell format for --temp: cmd (default) or powershell`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)
			appendPos := positionFromString(position)
			varName := args[0]
			value := strings.Join(args[1:], " ")

			if dryRun {
				fmt.Printf("%s %s '%s' to %s (position: %s, scope: %s)\n",
					ui.IconInfo, ui.Info("Would append"), ui.Path(value),
					ui.Highlight(varName), ui.Dim(appendPos.String()), ui.Dim(scopeType.String()))
				return nil
			}

			if temp {
				currentVar, err := m.Get(varName, scopeType)
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error reading %s: %v", varName, err)))
					return err
				}
				current := env.NormalizePathSeparator(currentVar.Value)
				for _, p := range strings.Split(current, string(os.PathListSeparator)) {
					if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(value)) {
						fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: value already exists in %s", varName)))
						return fmt.Errorf("value already exists in %s", varName)
					}
				}
				var newVal string
				if appendPos == env.AppendPre {
					newVal = value + string(os.PathListSeparator) + current
				} else {
					newVal = current + string(os.PathListSeparator) + value
				}
				newVal = env.NormalizePathSeparator(newVal)
				switch strings.ToLower(shell) {
				case "powershell", "ps", "pwsh":
					fmt.Printf("$env:%s = '%s'\n", varName, newVal)
				default:
					fmt.Printf("set %s=%s\n", varName, newVal)
				}
				return nil
			}

			if refresh {
				if err := m.AppendValueAndRefresh(varName, value, scopeType, appendPos); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
			} else {
				if err := m.AppendValue(varName, value, scopeType, appendPos); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
			}

			if clipboard {
				if v, err := m.Get(varName, scopeType); err == nil {
					if cerr := copyToClipboard(v.Value); cerr != nil {
						fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
					} else {
						fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Updated value copied to clipboard"))
					}
				}
			}

			fmt.Printf("%s %s '%s' to %s (%s %s)",
				ui.IconSuccess, ui.Success("Successfully appended"),
				ui.Path(value), ui.Highlight(varName),
				ui.Dim(appendPos.String()), ui.Dim(scopeType.String()))
			if refresh {
				fmt.Printf(" %s", ui.Info("[WM_SETTINGCHANGE broadcast]"))
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().StringVarP(&position, "position", "p", "post", "Append position: pre or post")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Broadcast WM_SETTINGCHANGE to GUI apps after registry write")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy updated variable value to clipboard")
	cmd.Flags().BoolVar(&temp, "temp", false, "Print shell command to update current terminal only (no registry write)")
	cmd.Flags().StringVar(&shell, "shell", "cmd", "Shell format for --temp output: cmd or powershell")
	return cmd
}

// ─── delete ─────────────────────────────────────────────────────────────────

func deleteCmd() *cobra.Command {
	var scope string
	var useRegex bool
	var confirm bool

	cmd := &cobra.Command{
		Use:     "delete [variable|pattern]",
		Aliases: []string{"d", "rm", "unset"},
		Short:   fmt.Sprintf("%s Delete environment variable", ui.IconDelete),
		Long: `Remove environment variables matching the given name or pattern.
Glob (* ? [abc]) and --regex patterns are supported.
When a pattern matches multiple variables, --confirm is required.

Examples:
  pathman delete MYVAR
  pathman delete "TEMP_*" --confirm
  pathman delete "^JAVA" --regex --confirm -s system`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			pat := args[0]
			scopeType := scopeFromString(scope)

			vars, err := matchVars(m, pat, scopeType, useRegex)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(err.Error()))
				return err
			}
			if len(vars) == 0 {
				fmt.Printf("%s No variables match %s\n", ui.IconWarning, ui.Warning(pattern.Describe(pat, useRegex)))
				return nil
			}

			// Safety: require --confirm when pattern hits multiple vars
			if len(vars) > 1 && !confirm {
				fmt.Printf("%s Pattern matches %d variables. Use --confirm to delete all:\n",
					ui.IconWarning, len(vars))
				for _, v := range vars {
					fmt.Printf("    %s %s (%s)\n", ui.IconMinus, ui.Highlight(v.Name), ui.Dim(v.Scope.String()))
				}
				return fmt.Errorf("use --confirm to delete %d variables", len(vars))
			}

			if dryRun {
				for _, v := range vars {
					fmt.Printf("%s %s '%s' (%s)\n",
						ui.IconInfo, ui.Info("Would delete"),
						ui.Highlight(v.Name), ui.Dim(scopeType.String()))
				}
				return nil
			}

			deleted := 0
			for _, v := range vars {
				if err := m.Delete(v.Name, scopeType); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error deleting %s: %v", v.Name, err)))
					continue
				}
				fmt.Printf("%s %s '%s' (%s)\n",
					ui.IconSuccess, ui.Success("Deleted"),
					ui.Highlight(v.Name), ui.Dim(scopeType.String()))
				deleted++
			}
			if len(vars) > 1 {
				fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("Deleted %d/%d variables", deleted, len(vars))))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat pattern as regex instead of glob")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm deletion when pattern matches multiple variables")
	return cmd
}

// ─── list ───────────────────────────────────────────────────────────────────

func listCmd() *cobra.Command {
	var scope string
	var clipboard bool
	var filter string
	var filterValue string
	var useRegex bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls", "all"},
		Short:   fmt.Sprintf("%s List all environment variables", ui.IconList),
		Long: `Display environment variables. Supports filtering by name and/or value.

  --filter/-F   glob/regex pattern matched against variable NAMES
  --value/-V    glob/regex pattern matched against variable VALUES
  --regex/-R    treat patterns as regex instead of glob
  -c            copy output to clipboard

Examples:
  pathman list --filter "*PATH*"
  pathman list --filter "JAVA*" -s system
  pathman list --value "C:\\Python*"
  pathman list --filter "*" --value "*mingw*" --regex`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeStr := strings.ToLower(scope)

			// Collect vars for requested scope(s)
			var allVars []env.Variable
			if scopeStr == "user" {
				vars, _ := m.List(env.ScopeUser)
				allVars = vars
			} else if scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
				vars, _ := m.List(env.ScopeSystem)
				allVars = vars
			} else {
				userVars, _ := m.List(env.ScopeUser)
				sysVars, _ := m.List(env.ScopeSystem)
				allVars = append(userVars, sysVars...)
			}

			// Apply name filter
			if filter != "" {
				var filtered []env.Variable
				for _, v := range allVars {
					ok, err := pattern.Match(filter, v.Name, useRegex)
					if err != nil {
						fmt.Printf("%s Invalid filter pattern: %v\n", ui.IconError, err)
						return err
					}
					if ok {
						filtered = append(filtered, v)
					}
				}
				allVars = filtered
			}

			// Apply value filter
			if filterValue != "" {
				var filtered []env.Variable
				for _, v := range allVars {
					ok, err := pattern.Match(filterValue, v.Value, useRegex)
					if err != nil {
						fmt.Printf("%s Invalid value pattern: %v\n", ui.IconError, err)
						return err
					}
					if ok {
						filtered = append(filtered, v)
					}
				}
				allVars = filtered
			}

			if len(allVars) == 0 {
				fmt.Printf("%s No variables match the given filters\n", ui.IconWarning)
				return nil
			}

			if format != "text" {
				output, err := env.FormatVariables(allVars, format)
				if err != nil {
					return err
				}
				if clipboard {
					copyToClipboard(output)
				}
				fmt.Print(output)
				return nil
			}

			var buf strings.Builder
			if filter != "" || filterValue != "" {
				// Filtered: show flat list regardless of scope grouping
				label := ""
				if filter != "" {
					label += "name:" + filter
				}
				if filterValue != "" {
					if label != "" {
						label += " "
					}
					label += "value:" + filterValue
				}
				buf.WriteString(fmt.Sprintf("\n%s Filtered variables  %s\n",
					ui.IconSearch, ui.Dim("("+label+")")))
				buf.WriteString(strings.Repeat("─", 80) + "\n")
				maxLen := 22
				for _, v := range allVars {
					if len(v.Name) > maxLen {
						maxLen = len(v.Name)
					}
				}
				maxLen += 3
				for _, v := range allVars {
					value := v.Value
					if len(value) > 55 {
						value = value[:52] + "..."
					}
					padding := strings.Repeat(" ", maxLen-len(v.Name))
					scopeTag := ui.Dim("[" + v.Scope.String() + "]")
					buf.WriteString(fmt.Sprintf("  %s%s %s %s  %s\n",
						ui.KeyValue(v.Name), padding, ui.Dim("="), ui.Path(value), scopeTag))
				}
				buf.WriteString(fmt.Sprintf("\n%s %s\n", ui.IconCheck,
					ui.Dim(fmt.Sprintf("%d variable(s) matched", len(allVars)))))
			} else if scopeStr == "user" || scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
				sType := env.ScopeUser
				if scopeStr != "user" {
					sType = env.ScopeSystem
				}
				appendVarSection(m, sType, &buf)
			} else {
				buf.WriteString(fmt.Sprintf("\n%s Environment Variables - Both Scopes\n", ui.HeaderInfo("🌍")))
				buf.WriteString(strings.Repeat("═", 80) + "\n")
				var userVars, sysVars []env.Variable
				for _, v := range allVars {
					if v.Scope == env.ScopeUser {
						userVars = append(userVars, v)
					} else {
						sysVars = append(sysVars, v)
					}
				}
				writeVarSection(ui.IconUser, "User", userVars, &buf)
				buf.WriteString("\n")
				writeVarSection(ui.IconSystem, "System", sysVars, &buf)
				buf.WriteString(fmt.Sprintf("\n%s %s\n", ui.IconCheck,
					ui.Dim(fmt.Sprintf("Total: %d variables (User + System)", len(allVars)))))
			}

			out := buf.String()
			if clipboard {
				if cerr := copyToClipboard(out); cerr != nil {
					fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
				} else {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Output copied to clipboard"))
				}
			}
			fmt.Print(out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "both", "Scope: user, system, or both")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy output to clipboard")
	cmd.Flags().StringVarP(&filter, "filter", "F", "", "Glob/regex pattern to filter by variable name")
	cmd.Flags().StringVarP(&filterValue, "value", "V", "", "Glob/regex pattern to filter by variable value")
	cmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat filter patterns as regex instead of glob")
	return cmd
}

func writeVarSection(icon, title string, vars []env.Variable, buf *strings.Builder) {
	buf.WriteString(fmt.Sprintf("\n%s %s Environment Variables:\n", icon, ui.HeaderInfo(title)))
	buf.WriteString(strings.Repeat("─", 80) + "\n")
	maxLen := 22
	for _, v := range vars {
		if len(v.Name) > maxLen {
			maxLen = len(v.Name)
		}
	}
	maxLen += 3
	for _, v := range vars {
		value := v.Value
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		padding := strings.Repeat(" ", maxLen-len(v.Name))
		buf.WriteString(fmt.Sprintf("  %s%s %s %s\n", ui.KeyValue(v.Name), padding, ui.Dim("="), ui.Path(value)))
	}
	buf.WriteString(fmt.Sprintf("  %s %s\n",
		ui.Dim(strings.Repeat("─", 78)),
		ui.Dim(fmt.Sprintf("%s: %d variables", title, len(vars)))))
}

func appendVarSection(m *env.Manager, scopeType env.Scope, buf *strings.Builder) error {
	vars, err := m.List(scopeType)
	if err != nil {
		fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
		return err
	}
	icon, title := ui.IconUser, "User"
	if scopeType == env.ScopeSystem {
		icon, title = ui.IconSystem, "System"
	}
	writeVarSection(icon, title, vars, buf)
	return nil
}

func printVarSection(icon, title string, vars []env.Variable) {
	var buf strings.Builder
	writeVarSection(icon, title, vars, &buf)
	fmt.Print(buf.String())
}

func printVarList(m *env.Manager, scopeType env.Scope) error {
	var buf strings.Builder
	return appendVarSection(m, scopeType, &buf)
}

// ─── path ───────────────────────────────────────────────────────────────────

func pathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "path",
		Aliases: []string{"p"},
		Short:   fmt.Sprintf("%s Manage PATH entries", ui.IconFolder),
		Long:    "Add (pre/post), remove, or list entries in the PATH variable",
	}

	// ── path add ──────────────────────────────────────────────────────────
	{
		var scope string
		var position string
		var refresh bool
		var clipboard bool

		addCmd := &cobra.Command{
			Use:     "add [directory]",
			Aliases: []string{"a", "append"},
			Short:   fmt.Sprintf("%s Add directory to PATH", ui.IconPlus),
			Long: `Add a new directory to the PATH variable.
Pattern matching is NOT applied here — the directory path must be literal.
  -s/--scope     user (default) or system
  -p/--position  pre (front) or post (end, default)
  -r/--refresh   broadcast WM_SETTINGCHANGE to GUI apps
  -c/--clipboard copy updated PATH to clipboard`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := scopeFromString(scope)
				appendPos := positionFromString(position)

				absPath, err := filepath.Abs(args[0])
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error resolving path: %v", err)))
					return err
				}

				if _, err := os.Stat(absPath); os.IsNotExist(err) {
					fmt.Printf("%s %s\n", ui.IconWarning,
						ui.Warning(fmt.Sprintf("Warning: Path does not exist: %s", absPath)))
				}

				if refresh {
					if err := m.PathAddAndRefresh("PATH", absPath, scopeType, appendPos, false); err != nil {
						fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
						return err
					}
				} else {
					if err := m.PathAdd("PATH", absPath, scopeType, appendPos, false); err != nil {
						fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
						return err
					}
				}

				if clipboard {
					if v, err := m.Get("PATH", scopeType); err == nil {
						copyToClipboard(v.Value)
						fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Updated PATH copied to clipboard"))
					}
				}

				posStr := "end of"
				if appendPos == env.AppendPre {
					posStr = "beginning of"
				}
				fmt.Printf("%s %s %s %s (%s)",
					ui.IconSuccess,
					ui.Success(fmt.Sprintf("Added to %s PATH:", posStr)),
					ui.Path(absPath),
					ui.Dim(scopeType.String()),
					ui.Dim(appendPos.String()),
				)
				if refresh {
					fmt.Printf(" %s", ui.Info("[WM_SETTINGCHANGE broadcast]"))
				}
				fmt.Println()
				return nil
			},
		}
		addCmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
		addCmd.Flags().StringVarP(&position, "position", "p", "post", "Add position: pre or post")
		addCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Broadcast WM_SETTINGCHANGE to GUI apps after registry write")
		addCmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy updated PATH to clipboard")
		cmd.AddCommand(addCmd)
	}

	// ── path remove ───────────────────────────────────────────────────────
	{
		var scope string
		var useRegex bool
		var confirm bool

		removeCmd := &cobra.Command{
			Use:     "remove [directory|pattern]",
			Aliases: []string{"rm", "delete"},
			Short:   fmt.Sprintf("%s Remove directory from PATH", ui.IconMinus),
			Long: `Remove entries from the PATH variable.
Accepts glob patterns (* ? [abc]) or --regex for regex.
When a pattern matches multiple entries, --confirm is required.

Examples:
  pathman path remove "C:\Python38"
  pathman path remove "C:\Python3*" --confirm
  pathman path remove "C:\\Python3[78]" --regex --confirm -s system`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := scopeFromString(scope)
				pat := args[0]

				entries, err := m.PathList("PATH", scopeType)
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}

				// Find matching entries
				var toRemove []env.PathEntry
				for _, e := range entries {
					ok, err := pattern.Match(pat, e.Value, useRegex)
					if err != nil {
						fmt.Printf("%s Invalid pattern: %v\n", ui.IconError, err)
						return err
					}
					if ok {
						toRemove = append(toRemove, e)
					}
				}

				if len(toRemove) == 0 {
					fmt.Printf("%s No PATH entries match %s\n", ui.IconWarning, ui.Warning(pattern.Describe(pat, useRegex)))
					return nil
				}

				if len(toRemove) > 1 && !confirm {
					fmt.Printf("%s Pattern matches %d PATH entries. Use --confirm to remove all:\n",
						ui.IconWarning, len(toRemove))
					for _, e := range toRemove {
						fmt.Printf("    %s %s\n", ui.IconMinus, ui.Path(e.Value))
					}
					return fmt.Errorf("use --confirm to remove %d entries", len(toRemove))
				}

				if dryRun {
					for _, e := range toRemove {
						fmt.Printf("%s %s %s\n", ui.IconInfo, ui.Info("Would remove:"), ui.Path(e.Value))
					}
					return nil
				}

				removed := 0
				for _, e := range toRemove {
					if err := m.PathRemove("PATH", e.Value, scopeType); err != nil {
						fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error removing %s: %v", e.Value, err)))
						continue
					}
					fmt.Printf("%s %s %s (%s)\n",
						ui.IconSuccess, ui.Success("Removed from PATH:"),
						ui.Path(e.Value), ui.Dim(scopeType.String()))
					removed++
				}
				if len(toRemove) > 1 {
					fmt.Printf("\n%s %s\n", ui.IconCheck,
						ui.Dim(fmt.Sprintf("Removed %d/%d entries", removed, len(toRemove))))
				}
				return nil
			},
		}
		removeCmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
		removeCmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat pattern as regex instead of glob")
		removeCmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm removal when pattern matches multiple entries")
		cmd.AddCommand(removeCmd)
	}

	// ── path list ─────────────────────────────────────────────────────────
	{
		var scope string
		var clipboard bool
		var filter string
		var useRegex bool

		listPathCmd := &cobra.Command{
			Use:     "list",
			Aliases: []string{"ls", "show"},
			Short:   fmt.Sprintf("%s List PATH entries", ui.IconList),
			Long: `Display all entries in the PATH variable.
  --filter/-F   glob/regex pattern to filter entries by path string
  --regex/-R    treat filter as regex instead of glob
  -c            copy plain-text list to clipboard

Examples:
  pathman path list
  pathman path list --filter "C:\Python*"
  pathman path list --filter "mingw" --regex -s system`,
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := scopeFromString(scope)
				entries, err := m.PathList("PATH", scopeType)
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}

				// Apply filter
				if filter != "" {
					var filtered []env.PathEntry
					for _, e := range entries {
						ok, err := pattern.Match(filter, e.Value, useRegex)
						if err != nil {
							fmt.Printf("%s Invalid filter: %v\n", ui.IconError, err)
							return err
						}
						if ok {
							filtered = append(filtered, e)
						}
					}
					entries = filtered
				}

				if len(entries) == 0 {
					fmt.Printf("%s No PATH entries match filter\n", ui.IconWarning)
					return nil
				}

				var colored strings.Builder
				var plain strings.Builder
				header := fmt.Sprintf("%s PATH Entries", scopeType.String())
				if filter != "" {
					header += "  (" + pattern.Describe(filter, useRegex) + ")"
				}
				colored.WriteString(fmt.Sprintf("\n%s %s PATH Entries",
					ui.GetScopeIcon(scope), ui.HeaderInfo(scopeType.String())))
				if filter != "" {
					colored.WriteString("  " + ui.Dim("("+pattern.Describe(filter, useRegex)+")"))
				}
				colored.WriteString("\n" + strings.Repeat("─", 80) + "\n")
				plain.WriteString(header + "\n" + strings.Repeat("-", 60) + "\n")

				for _, entry := range entries {
					status := ui.IconCheck
					plainStatus := "OK "
					if !entry.Exist {
						status = ui.IconBroken
						plainStatus = "ERR"
					}
					colored.WriteString(fmt.Sprintf("  %s [%3d] %s\n", status, entry.Index, ui.Path(entry.Value)))
					plain.WriteString(fmt.Sprintf("[%s] %3d  %s\n", plainStatus, entry.Index, entry.Value))
				}
				if filter != "" {
					colored.WriteString(fmt.Sprintf("\n%s %s\n", ui.IconCheck,
						ui.Dim(fmt.Sprintf("%d entries matched", len(entries)))))
					plain.WriteString(fmt.Sprintf("\n%d entries matched\n", len(entries)))
				}

				if clipboard {
					if cerr := copyToClipboard(plain.String()); cerr != nil {
						fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
					} else {
						fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("PATH entries copied to clipboard"))
					}
				}
				fmt.Print(colored.String())
				return nil
			},
		}
		listPathCmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
		listPathCmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy PATH entries (plain text) to clipboard")
		listPathCmd.Flags().StringVarP(&filter, "filter", "F", "", "Glob/regex pattern to filter entries")
		listPathCmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat filter as regex instead of glob")
		cmd.AddCommand(listPathCmd)
	}

	return cmd
}

// ─── parse ──────────────────────────────────────────────────────────────────

func parseCmd() *cobra.Command {
	var scope string
	var clipboard bool
	var useRegex bool

	cmd := &cobra.Command{
		Use:     "parse [variable|pattern]",
		Aliases: []string{"format", "export", "fmt"},
		Short:   fmt.Sprintf("%s Parse variable to different format", ui.IconRefresh),
		Long: `Output variable(s) in specified format (json, yaml, csv, table).
Accepts glob patterns or --regex for multiple variable output.
Use -c to copy to clipboard.

Examples:
  pathman parse PATH -f json
  pathman parse "JAVA*" -f table
  pathman parse "*" -f csv -s system`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			pat := args[0]
			scopeType := scopeFromString(scope)

			vars, err := matchVars(m, pat, scopeType, useRegex)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			if len(vars) == 0 {
				fmt.Printf("%s No variables match %s\n", ui.IconWarning, ui.Warning(pattern.Describe(pat, useRegex)))
				return nil
			}

			output, err := env.FormatVariables(vars, format)
			if err != nil {
				return err
			}
			if clipboard {
				if cerr := copyToClipboard(output); cerr != nil {
					fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
				} else {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Output copied to clipboard"))
				}
			}
			fmt.Print(output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy formatted output to clipboard")
	cmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat pattern as regex instead of glob")
	return cmd
}

// ─── clean ──────────────────────────────────────────────────────────────────

func cleanCmd() *cobra.Command {
	var scope string

	cmd := &cobra.Command{
		Use:     "clean",
		Aliases: []string{"cleanup", "dedupe"},
		Short:   fmt.Sprintf("%s Clean up PATH variable", ui.IconRefresh),
		Long:    "Remove duplicates and non-existent paths from PATH",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)
			entries, err := m.PathList("PATH", scopeType)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			seen := make(map[string]bool)
			var cleaned []string
			duplicates, nonexistent := 0, 0
			fmt.Printf("\n%s %s PATH Analysis:\n", ui.GetScopeIcon(scope), ui.HeaderInfo(scopeType.String()))
			fmt.Println(strings.Repeat("─", 80))
			for _, entry := range entries {
				lower := strings.ToLower(entry.Value)
				if seen[lower] {
					fmt.Printf("  %s %s (duplicate)\n", ui.IconCross, ui.Warning(entry.Value))
					duplicates++
					continue
				}
				if !entry.Exist {
					fmt.Printf("  %s %s (not found)\n", ui.IconWarning, ui.Warning(entry.Value))
					nonexistent++
					continue
				}
				fmt.Printf("  %s %s\n", ui.IconCheck, ui.Path(entry.Value))
				cleaned = append(cleaned, entry.Value)
				seen[lower] = true
			}
			if dryRun {
				fmt.Printf("\n%s %s\n", ui.IconInfo,
					ui.Info(fmt.Sprintf("Would remove %d duplicates and %d non-existent paths", duplicates, nonexistent)))
				return nil
			}
			if duplicates > 0 || nonexistent > 0 {
				newPath := strings.Join(cleaned, string(os.PathListSeparator))
				if err := m.Set("PATH", newPath, scopeType); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
				fmt.Printf("\n%s %s\n", ui.IconSuccess,
					ui.Success(fmt.Sprintf("PATH cleaned: removed %d duplicates and %d non-existent paths",
						duplicates, nonexistent)))
			} else {
				fmt.Printf("\n%s %s\n", ui.IconSuccess, ui.Success("PATH is already clean!"))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	return cmd
}

// ─── info ───────────────────────────────────────────────────────────────────

func infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info",
		Aliases: []string{"i", "about", "version"},
		Short:   fmt.Sprintf("%s Show environment information", ui.IconInfo),
		Long:    "Display current environment configuration and paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("\n%s PathMan v1.0.0\n", ui.IconRocket)
			fmt.Println(strings.Repeat("═", 60))
			fmt.Printf("\n%s %s\n", ui.IconGear, ui.HeaderInfo("System Info"))
			fmt.Printf("  %s %s %s\n", ui.IconUser, ui.Info("User:"), os.Getenv("USERNAME"))
			fmt.Printf("  %s %s %s\n", ui.IconSystem, ui.Info("Computer:"), os.Getenv("COMPUTERNAME"))
			fmt.Printf("  %s %s %s\n", ui.IconFolder, ui.Info("Home:"), os.Getenv("USERPROFILE"))
			fmt.Printf("  %s %s %s\n", ui.IconGear, ui.Info("OS:"), os.Getenv("OS"))
			fmt.Printf("\n%s %s\n", ui.IconStar, ui.HeaderInfo("Important Paths"))
			fmt.Printf("  %s %s %s\n", ui.IconFolder, ui.Info("System Root:"), os.Getenv("SystemRoot"))
			fmt.Printf("  %s %s %s\n", ui.IconFolder, ui.Info("Program Files:"), os.Getenv("ProgramFiles"))
			fmt.Printf("  %s %s %s\n", ui.IconFolder, ui.Info("AppData:"), os.Getenv("APPDATA"))
			fmt.Printf("  %s %s %s\n", ui.IconFolder, ui.Info("Temp:"), os.Getenv("TEMP"))
			fmt.Printf("\n%s %s\n", ui.IconInfo, ui.Dim("Use 'pathman --help' for available commands"))
			return nil
		},
	}
}

// ─── remove-value ───────────────────────────────────────────────────────────

func removeValueCmd() *cobra.Command {
	var scope string
	var refresh bool
	var useRegex bool
	var confirm bool

	cmd := &cobra.Command{
		Use:     "remove-value [variable] [value|pattern]",
		Aliases: []string{"rv", "remove", "unappend"},
		Short:   fmt.Sprintf("%s Remove value from variable", ui.IconMinus),
		Long: `Remove one or more values from a semicolon-separated variable.
The value argument accepts glob patterns or --regex.
When a pattern matches multiple entries, --confirm is required.

Examples:
  pathman remove-value PATH "C:\Python38"
  pathman remove-value PATH "C:\Python3*" --confirm
  pathman remove-value PYTHONPATH "*site-packages*" --confirm
  pathman remove-value PATH "Python3[78]" --regex --confirm`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)
			varName := args[0]
			pat := args[1]

			v, err := m.Get(varName, scopeType)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}

			parts := strings.Split(env.NormalizePathSeparator(v.Value), string(os.PathListSeparator))
			var toRemove []string
			var toKeep []string
			for _, p := range parts {
				if p == "" {
					continue
				}
				ok, err := pattern.Match(pat, p, useRegex)
				if err != nil {
					fmt.Printf("%s Invalid pattern: %v\n", ui.IconError, err)
					return err
				}
				if ok {
					toRemove = append(toRemove, p)
				} else {
					toKeep = append(toKeep, p)
				}
			}

			if len(toRemove) == 0 {
				fmt.Printf("%s No values match %s in %s\n",
					ui.IconWarning, ui.Warning(pattern.Describe(pat, useRegex)), ui.Highlight(varName))
				return nil
			}

			if len(toRemove) > 1 && !confirm {
				fmt.Printf("%s Pattern matches %d values in %s. Use --confirm to remove all:\n",
					ui.IconWarning, len(toRemove), ui.Highlight(varName))
				for _, p := range toRemove {
					fmt.Printf("    %s %s\n", ui.IconMinus, ui.Path(p))
				}
				return fmt.Errorf("use --confirm to remove %d values", len(toRemove))
			}

			if dryRun {
				for _, p := range toRemove {
					fmt.Printf("%s %s '%s' from %s\n",
						ui.IconInfo, ui.Info("Would remove"), ui.Path(p), ui.Highlight(varName))
				}
				return nil
			}

			newValue := env.NormalizePathSeparator(strings.Join(toKeep, string(os.PathListSeparator)))
			if err := m.Set(varName, newValue, scopeType); err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}

			for _, p := range toRemove {
				fmt.Printf("%s Removed '%s' from %s (%s)\n",
					ui.IconSuccess, ui.Path(p), ui.Highlight(varName), ui.Dim(scopeType.String()))
			}
			if len(toRemove) > 1 {
				fmt.Printf("\n%s %s\n", ui.IconCheck,
					ui.Dim(fmt.Sprintf("Removed %d values from %s", len(toRemove), varName)))
			}

			if refresh {
				if rerr := m.SetAndRefresh(varName, newValue, scopeType); rerr != nil {
					fmt.Printf("%s %s\n", ui.IconWarning,
						ui.Warning(fmt.Sprintf("Refresh warning: %v", rerr)))
				} else {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("[WM_SETTINGCHANGE broadcast]"))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Broadcast WM_SETTINGCHANGE to GUI apps after registry write")
	cmd.Flags().BoolVarP(&useRegex, "regex", "R", false, "Treat value pattern as regex instead of glob")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm when pattern matches multiple values")
	return cmd
}

// ─── refresh ────────────────────────────────────────────────────────────────

func refreshCmd() *cobra.Command {
	var clipboard bool

	cmd := &cobra.Command{
		Use:     "refresh",
		Aliases: []string{"reload", "sync", "update-path"},
		Short:   fmt.Sprintf("%s Refresh PATH in current terminal from registry", ui.IconRefresh),
		Long: `Read System + User PATH from the Windows registry and broadcast
WM_SETTINGCHANGE to notify GUI apps (Explorer, some IDEs) to reload env.

NOTE: This cannot update the PATH of your current cmd/PowerShell session.
Windows does not allow a child process to write into its parent's environment.
To update the current terminal, use --temp on append/path add:
  for /f "delims=" %i in ('pathman append PATH "C:\Tool" --temp') do @%i
  Invoke-Expression (pathman append PATH "C:\Tool" --temp --shell powershell)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)

			if dryRun {
				fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Would refresh PATH from registry (dry-run)"))
				return nil
			}

			sysPath, sysErr := m.Get("PATH", env.ScopeSystem)
			userPath, userErr := m.Get("PATH", env.ScopeUser)

			if sysErr != nil && userErr != nil {
				err := fmt.Errorf("could not read PATH from registry: sys=%v user=%v", sysErr, userErr)
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(err.Error()))
				return err
			}

			sysVal := ""
			if sysErr == nil {
				sysVal = sysPath.Value
			}
			userVal := ""
			if userErr == nil {
				userVal = userPath.Value
			}

			combined := sysVal
			if userVal != "" {
				if combined != "" {
					combined += ";" + userVal
				} else {
					combined = userVal
				}
			}

			os.Setenv("PATH", combined)

			if err := m.BroadcastChange(); err != nil {
				fmt.Printf("%s %s\n", ui.IconWarning,
					ui.Warning(fmt.Sprintf("Broadcast warning: %v", err)))
			}

			if clipboard {
				if cerr := copyToClipboard(combined); cerr != nil {
					fmt.Printf("%s %s\n", ui.IconWarning,
						ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
				} else {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Refreshed PATH copied to clipboard"))
				}
			}

			countEntries := func(s string) int {
				s = strings.TrimSpace(strings.Trim(s, ";"))
				if s == "" {
					return 0
				}
				return len(strings.Split(s, ";"))
			}

			fmt.Printf("%s %s\n", ui.IconSuccess, ui.Success("PATH refreshed from registry"))
			fmt.Printf("  %s System entries: %d\n", ui.IconSystem, countEntries(sysVal))
			fmt.Printf("  %s User entries:   %d\n", ui.IconUser, countEntries(userVal))
			fmt.Printf("  %s Combined total: %d entries\n", ui.IconCheck, countEntries(combined))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy refreshed PATH to clipboard")
	return cmd
}
