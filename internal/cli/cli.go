/*
 FILE: internal/cli/cli.go

 Changes from original:
   1. Per-command local flag variables — no package-level scope/refresh/position/clipboard
      shared state that caused wrong defaults and cross-command pollution.
   2. path add/remove/list now properly register -s, -r, -p, -c flags on themselves.
   3. setCmd: removed fragile string-stripping hack; cobra parses flags correctly before RunE.
   4. PathAdd / PathAddAndRefresh: new signature passes AppendPosition and checkExist=false.
   5. Added -c/--clipboard flag to: get, set, append, list, path add, path list, parse.
   6. Added top-level "refresh" command.
   7. remove-value -r: uses SetAndRefresh (broadcasts WM_SETTINGCHANGE) not bare os.Setenv.
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

// copyToClipboard pipes text into the Windows built-in "clip" command.
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
Shows both User and System scopes by default.

Features:
  • Add, remove, and list PATH entries
  • Set, get, and delete environment variables
  • Append to variables with pre/post position
  • User and System scope management
  • Duplicate detection and cleanup
  • Non-existent path validation
  • JSON, YAML, CSV, Table output formats
  • Dry-run mode for safe testing
  • Clipboard copy with -c/--clipboard
  • Instant terminal refresh with -r/--refresh
        Flags work before OR after the value:
        pathman append PATH "val" -s system -r -c`,
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

// ─── get ────────────────────────────────────────────────────────────────────

func getCmd() *cobra.Command {
	var scope string
	var clipboard bool

	cmd := &cobra.Command{
		Use:     "get [variable]",
		Aliases: []string{"g", "show", "value"},
		Short:   fmt.Sprintf("%s Get environment variable value", ui.IconSearch),
		Long:    "Retrieve and display the value of an environment variable. Use -c to copy value to clipboard.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)
			v, err := m.Get(args[0], scopeType)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(err.Error()))
				return err
			}
			if clipboard {
				if cerr := copyToClipboard(v.Value); cerr != nil {
					fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
				} else {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Value copied to clipboard"))
				}
			}
			if format != "text" {
				output, _ := env.FormatVariable(v, format)
				fmt.Print(output)
				return nil
			}
			fmt.Printf("\n%s %s (%s)\n  %s\n",
				ui.GetScopeIcon(scope), ui.Highlight(v.Name),
				ui.Dim(scopeType.String()), ui.Path(v.Value))
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy value to clipboard")
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
Cobra parses flags correctly before RunE — no manual string stripping needed.
  -s/--scope     user (default) or system
  -r/--refresh   update current terminal session immediately
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
				fmt.Printf(" %s", ui.Info("[terminal updated]"))
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Update current terminal session immediately")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy new value to clipboard")
	return cmd
}

// ─── append ─────────────────────────────────────────────────────────────────

func appendCmd() *cobra.Command {
	var scope string
	var position string
	var refresh bool
	var clipboard bool

	cmd := &cobra.Command{
		Use:     "append [variable] [value]",
		Aliases: []string{"app", "concat", "add-to", "a", "add"},
		Short:   fmt.Sprintf("%s Append value to variable", ui.IconPlus),
		Long: `Append value to existing variable at beginning (pre) or end (post).
Duplicates are detected and prevented.
Flags work before OR after the value:
  pathman append PATH "C:\MyTool" -s system -r -c`,
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
				fmt.Printf(" %s", ui.Info("[terminal updated]"))
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().StringVarP(&position, "position", "p", "post", "Append position: pre or post")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Update current terminal session immediately")
	cmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy updated variable value to clipboard")
	return cmd
}

// ─── delete ─────────────────────────────────────────────────────────────────

func deleteCmd() *cobra.Command {
	var scope string

	cmd := &cobra.Command{
		Use:     "delete [variable]",
		Aliases: []string{"d", "rm", "unset"},
		Short:   fmt.Sprintf("%s Delete environment variable", ui.IconDelete),
		Long:    "Remove an entire environment variable from the specified scope",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)

			if dryRun {
				fmt.Printf("%s %s '%s' (%s %s)\n",
					ui.IconInfo, ui.Info("Would delete"),
					ui.Highlight(args[0]),
					ui.GetScopeIcon(scope), ui.Dim(scopeType.String()))
				return nil
			}

			if err := m.Delete(args[0], scopeType); err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}

			fmt.Printf("%s %s '%s' (%s %s)\n",
				ui.IconSuccess, ui.Success("Successfully deleted"),
				ui.Highlight(args[0]),
				ui.GetScopeIcon(scope), ui.Dim(scopeType.String()))
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	return cmd
}

// ─── list ───────────────────────────────────────────────────────────────────

func listCmd() *cobra.Command {
	var scope string
	var clipboard bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls", "all"},
		Short:   fmt.Sprintf("%s List all environment variables", ui.IconList),
		Long:    "Display all environment variables (default: both user and system). Use -c to copy to clipboard.",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeStr := strings.ToLower(scope)

			if format != "text" {
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
				output, err := env.FormatVariables(allVars, format)
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
			}

			var buf strings.Builder
			if scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
				if err := appendVarSection(m, env.ScopeSystem, &buf); err != nil {
					return err
				}
			} else if scopeStr == "user" {
				if err := appendVarSection(m, env.ScopeUser, &buf); err != nil {
					return err
				}
			} else {
				buf.WriteString(fmt.Sprintf("\n%s Environment Variables - Both Scopes\n", ui.HeaderInfo("🌍")))
				buf.WriteString(strings.Repeat("═", 80) + "\n")
				userVars, _ := m.List(env.ScopeUser)
				sysVars, _ := m.List(env.ScopeSystem)
				writeVarSection(ui.IconUser, "User", userVars, &buf)
				buf.WriteString("\n")
				writeVarSection(ui.IconSystem, "System", sysVars, &buf)
				buf.WriteString(fmt.Sprintf("\n%s %s\n", ui.IconCheck,
					ui.Dim(fmt.Sprintf("Total: %d variables (User + System)", len(userVars)+len(sysVars)))))
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
  -s/--scope     user (default) or system
  -p/--position  pre (front) or post (end, default)
  -r/--refresh   update current terminal immediately
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

				// Warn about non-existent paths but still allow adding.
				// checkExist=false so PathAdd doesn't double-error after our warning.
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
						if cerr := copyToClipboard(v.Value); cerr != nil {
							fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Clipboard error: %v", cerr)))
						} else {
							fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Updated PATH copied to clipboard"))
						}
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
					fmt.Printf(" %s", ui.Info("[terminal updated]"))
				}
				fmt.Println()
				return nil
			},
		}
		addCmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
		addCmd.Flags().StringVarP(&position, "position", "p", "post", "Add position: pre or post")
		addCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Update current terminal session immediately")
		addCmd.Flags().BoolVarP(&clipboard, "clipboard", "c", false, "Copy updated PATH to clipboard")
		cmd.AddCommand(addCmd)
	}

	// ── path remove ───────────────────────────────────────────────────────
	{
		var scope string

		removeCmd := &cobra.Command{
			Use:     "remove [directory]",
			Aliases: []string{"rm", "delete"},
			Short:   fmt.Sprintf("%s Remove directory from PATH", ui.IconMinus),
			Long:    "Remove a directory from the PATH variable",
			Args:    cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := scopeFromString(scope)
				if err := m.PathRemove("PATH", args[0], scopeType); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
				fmt.Printf("%s %s %s (%s)\n",
					ui.IconSuccess, ui.Success("Removed from PATH:"),
					ui.Path(args[0]), ui.Dim(scopeType.String()))
				return nil
			},
		}
		removeCmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
		cmd.AddCommand(removeCmd)
	}

	// ── path list ─────────────────────────────────────────────────────────
	{
		var scope string
		var clipboard bool

		listPathCmd := &cobra.Command{
			Use:     "list",
			Aliases: []string{"ls", "show"},
			Short:   fmt.Sprintf("%s List PATH entries", ui.IconList),
			Long:    "Display all entries in the PATH variable. Use -c to copy plain-text list to clipboard.",
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := scopeFromString(scope)
				entries, err := m.PathList("PATH", scopeType)
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}

				var colored strings.Builder
				var plain strings.Builder
				colored.WriteString(fmt.Sprintf("\n%s %s PATH Entries:\n",
					ui.GetScopeIcon(scope), ui.HeaderInfo(scopeType.String())))
				colored.WriteString(strings.Repeat("─", 80) + "\n")
				plain.WriteString(fmt.Sprintf("%s PATH Entries:\n%s\n",
					scopeType.String(), strings.Repeat("-", 60)))

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
		cmd.AddCommand(listPathCmd)
	}

	return cmd
}

// ─── parse ──────────────────────────────────────────────────────────────────

func parseCmd() *cobra.Command {
	var scope string
	var clipboard bool

	cmd := &cobra.Command{
		Use:     "parse [variable]",
		Aliases: []string{"format", "export", "fmt"},
		Short:   fmt.Sprintf("%s Parse variable to different format", ui.IconRefresh),
		Long:    "Output variable in specified format (json, yaml, csv, table). Use -c to copy to clipboard.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)
			v, err := m.Get(args[0], scopeType)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			output, err := env.FormatVariable(v, format)
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

	cmd := &cobra.Command{
		Use:     "remove-value [variable] [value]",
		Aliases: []string{"rv", "remove", "unappend"},
		Short:   fmt.Sprintf("%s Remove value from variable", ui.IconMinus),
		Long:    "Remove a specific value from a semicolon-separated variable",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := scopeFromString(scope)

			if dryRun {
				fmt.Printf("%s Would remove '%s' from %s (%s)\n",
					ui.IconInfo, ui.Path(args[1]), ui.Highlight(args[0]), ui.Dim(scopeType.String()))
				return nil
			}

			if err := m.PathRemove(args[0], args[1], scopeType); err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}

			fmt.Printf("%s Removed '%s' from %s (%s)\n",
				ui.IconSuccess, ui.Path(args[1]), ui.Highlight(args[0]), ui.Dim(scopeType.String()))

			if refresh {
				// SetAndRefresh broadcasts WM_SETTINGCHANGE — bare os.Setenv does not.
				if v, err := m.Get(args[0], scopeType); err == nil {
					if rerr := m.SetAndRefresh(args[0], v.Value, scopeType); rerr != nil {
						fmt.Printf("%s %s\n", ui.IconWarning,
							ui.Warning(fmt.Sprintf("Refresh warning: %v", rerr)))
					} else {
						fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("[terminal updated]"))
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Update current terminal session immediately")
	return cmd
}

// ─── refresh ────────────────────────────────────────────────────────────────

// refreshCmd rebuilds the current process PATH from the Windows registry,
// combining System PATH + User PATH exactly as a new terminal session would,
// then broadcasts WM_SETTINGCHANGE so Explorer and other apps pick up the change.
func refreshCmd() *cobra.Command {
	var clipboard bool

	cmd := &cobra.Command{
		Use:     "refresh",
		Aliases: []string{"reload", "sync", "update-path"},
		Short:   fmt.Sprintf("%s Refresh PATH in current terminal from registry", ui.IconRefresh),
		Long: `Rebuild the current terminal's PATH by reading System + User PATH from the
Windows registry and broadcasting WM_SETTINGCHANGE to all windows.
Run this after any tool has modified environment variables outside of pathman.
  -c/--clipboard  copy the refreshed PATH to clipboard`,
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
