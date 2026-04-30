package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/cumulus13/pathman/internal/env"
	"github.com/cumulus13/pathman/internal/ui"
)

var (
	manager  *env.Manager
	rootCmd  *cobra.Command
	
	// Global flags
	scope    string
	dryRun   bool
	noColor  bool
	format   string  // Added: output format
)


// Execute runs the CLI application
func Execute() error {
	return rootCmd.Execute()
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
  • User and System scope management
  • Duplicate detection and cleanup
  • Non-existent path validation
  • Dry-run mode for safe testing
  • JSON, YAML, CSV output formats`,
		Version: "1.0.0",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				color.NoColor = true
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&scope, "scope", "s", "both", "Scope: user, system, or both")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "text", "Output format: text, json, yaml, csv")

	rootCmd.AddCommand(getCmd())
	rootCmd.AddCommand(setCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(pathCmd())
	rootCmd.AddCommand(cleanCmd())
	rootCmd.AddCommand(infoCmd())
}

func parseScope() env.Scope {
	switch strings.ToLower(scope) {
	case "system", "machine", "global":
		return env.ScopeSystem
	case "both", "all", "":
		return env.ScopeUser  // Default, but handled in list command
	default:
		return env.ScopeUser
	}
}

func isBothScopes() bool {
	return strings.ToLower(scope) == "both" || strings.ToLower(scope) == "all"
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get [variable]",
		Aliases: []string{"g", "show", "value"},
		Short:   fmt.Sprintf("%s Get environment variable value", ui.IconSearch),
		Long:    "Retrieve and display the value of an environment variable from user, system, or both scopes",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeStr := strings.ToLower(scope)

			// Handle output format
			if format != "text" {
				return getFormattedOutput(m, args[0], scopeStr, format)
			}

			// Show both scopes (text mode)
			if scopeStr != "user" && scopeStr != "system" && scopeStr != "machine" && scopeStr != "global" {
				fmt.Printf("\n%s %s - Both Scopes\n", ui.IconSearch, ui.Highlight(args[0]))
				fmt.Println(strings.Repeat("═", 80))
				
				// User scope
				userVar, err := m.Get(args[0], env.ScopeUser)
				if err != nil {
					fmt.Printf("%s %s: %s\n", ui.IconUser, ui.Dim("User"), ui.Error(fmt.Sprintf("Not found (%v)", err)))
				} else {
					fmt.Printf("%s %s:\n", ui.IconUser, ui.HeaderInfo("User"))
					fmt.Printf("  %s\n", ui.Path(userVar.Value))
				}
				
				fmt.Println()
				
				// System scope
				sysVar, err := m.Get(args[0], env.ScopeSystem)
				if err != nil {
					fmt.Printf("%s %s: %s\n", ui.IconSystem, ui.Dim("System"), ui.Error(fmt.Sprintf("Not found (%v)", err)))
				} else {
					fmt.Printf("%s %s:\n", ui.IconSystem, ui.HeaderInfo("System"))
					fmt.Printf("  %s\n", ui.Path(sysVar.Value))
				}
				
				return nil
			}

			// Single scope (text mode)
			scopeType := parseScope()
			v, err := m.Get(args[0], scopeType)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}

			fmt.Printf("\n%s %s (%s)\n", ui.GetScopeIcon(scope), ui.Highlight(v.Name), ui.Dim(scopeType.String()))
			fmt.Printf("  %s\n", ui.Path(v.Value))

			return nil
		},
	}
}

// getFormattedOutput handles non-text output formats
func getFormattedOutput(m *env.Manager, varName, scopeStr, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return outputJSON(m, varName, scopeStr)
	case "csv":
		return outputCSV(m, varName, scopeStr)
	case "yaml", "yml":
		return outputYAML(m, varName, scopeStr)
	default:
		return fmt.Errorf("unsupported format: %s (use: text, json, yaml, csv)", format)
	}
}

func outputJSON(m *env.Manager, varName, scopeStr string) error {
	var allVars []env.Variable
	
	if scopeStr == "user" {
		v, err := m.Get(varName, env.ScopeUser)
		if err != nil {
			return err
		}
		allVars = append(allVars, *v)
	} else if scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
		v, err := m.Get(varName, env.ScopeSystem)
		if err != nil {
			return err
		}
		allVars = append(allVars, *v)
	} else {
		// Both scopes
		if v, err := m.Get(varName, env.ScopeUser); err == nil {
			allVars = append(allVars, *v)
		}
		if v, err := m.Get(varName, env.ScopeSystem); err == nil {
			allVars = append(allVars, *v)
		}
	}
	
	jsonStr, err := env.VariablesToJSON(allVars, scopeStr)
	if err != nil {
		return err
	}
	
	fmt.Print(jsonStr)
	return nil
}

func outputYAML(m *env.Manager, varName, scopeStr string) error {
	var allVars []env.Variable
	
	if scopeStr == "user" {
		v, err := m.Get(varName, env.ScopeUser)
		if err != nil {
			return err
		}
		allVars = append(allVars, *v)
	} else if scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
		v, err := m.Get(varName, env.ScopeSystem)
		if err != nil {
			return err
		}
		allVars = append(allVars, *v)
	} else {
		if v, err := m.Get(varName, env.ScopeUser); err == nil {
			allVars = append(allVars, *v)
		}
		if v, err := m.Get(varName, env.ScopeSystem); err == nil {
			allVars = append(allVars, *v)
		}
	}
	
	fmt.Print(env.VariablesToYAML(allVars))
	return nil
}

func outputCSV(m *env.Manager, varName, scopeStr string) error {
	var allVars []env.Variable
	
	if scopeStr == "user" {
		v, err := m.Get(varName, env.ScopeUser)
		if err != nil {
			return err
		}
		allVars = append(allVars, *v)
	} else if scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
		v, err := m.Get(varName, env.ScopeSystem)
		if err != nil {
			return err
		}
		allVars = append(allVars, *v)
	} else {
		if v, err := m.Get(varName, env.ScopeUser); err == nil {
			allVars = append(allVars, *v)
		}
		if v, err := m.Get(varName, env.ScopeSystem); err == nil {
			allVars = append(allVars, *v)
		}
	}
	
	csvStr, err := env.VariablesToCSV(allVars)
	if err != nil {
		return err
	}
	
	fmt.Print(csvStr)
	return nil
}

func setCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "set [variable] [value]",
		Aliases: []string{"s", "add"},
		Short:   fmt.Sprintf("%s Set environment variable", ui.IconSave),
		Long:    "Create or update an environment variable with the specified value",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := parseScope()
			
			if dryRun {
				fmt.Printf("%s %s %s=%s (scope: %s)\n",
					ui.IconInfo,
					ui.Info("Would set"),
					ui.Highlight(args[0]),
					ui.Path(args[1]),
					ui.Dim(scopeType.String()),
				)
				return nil
			}
			
			if err := m.Set(args[0], args[1], scopeType); err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			
			fmt.Printf("%s %s %s=%s (%s %s)\n",
				ui.IconSuccess,
				ui.Success("Successfully set"),
				ui.Highlight(args[0]),
				ui.Path(args[1]),
				ui.GetScopeIcon(scope),
				ui.Dim(scopeType.String()),
			)
			
			return nil
		},
	}
}

func deleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete [variable]",
		Aliases: []string{"d", "rm", "remove", "unset"},
		Short:   fmt.Sprintf("%s Delete environment variable", ui.IconDelete),
		Long:    "Remove an environment variable from the specified scope",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := parseScope()
			
			if dryRun {
				fmt.Printf("%s %s '%s' (%s %s)\n",
					ui.IconInfo,
					ui.Info("Would delete"),
					ui.Highlight(args[0]),
					ui.GetScopeIcon(scope),
					ui.Dim(scopeType.String()),
				)
				return nil
			}
			
			if err := m.Delete(args[0], scopeType); err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			
			fmt.Printf("%s %s '%s' (%s %s)\n",
				ui.IconSuccess,
				ui.Success("Successfully deleted"),
				ui.Highlight(args[0]),
				ui.GetScopeIcon(scope),
				ui.Dim(scopeType.String()),
			)
			
			return nil
		},
	}
}

func listFormattedOutput(m *env.Manager, scopeStr, format string) error {
	var allVars []env.Variable
	
	if scopeStr == "user" {
		vars, err := m.List(env.ScopeUser)
		if err != nil {
			return err
		}
		allVars = vars
	} else if scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
		vars, err := m.List(env.ScopeSystem)
		if err != nil {
			return err
		}
		allVars = vars
	} else {
		userVars, _ := m.List(env.ScopeUser)
		sysVars, _ := m.List(env.ScopeSystem)
		allVars = append(userVars, sysVars...)
	}
	
	switch strings.ToLower(format) {
	case "json":
		jsonStr, err := env.VariablesToJSON(allVars, scopeStr)
		if err != nil {
			return err
		}
		fmt.Print(jsonStr)
	case "yaml", "yml":
		fmt.Print(env.VariablesToYAML(allVars))
	case "csv":
		csvStr, err := env.VariablesToCSV(allVars)
		if err != nil {
			return err
		}
		fmt.Print(csvStr)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	
	return nil
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls", "all"},
		Short:   fmt.Sprintf("%s List all environment variables", ui.IconList),
		Long:    "Display all environment variables (default: both user and system)",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeStr := strings.ToLower(scope)

			// Handle non-text formats
			if format != "text" {
				return listFormattedOutput(m, scopeStr, format)
			}

			if scopeStr == "system" || scopeStr == "machine" || scopeStr == "global" {
				return printVarList(m, env.ScopeSystem)
			} else if scopeStr == "user" {
				return printVarList(m, env.ScopeUser)
			} else {
				// DEFAULT: Show both with nice header
				fmt.Printf("\n%s Environment Variables - Both Scopes\n", ui.HeaderInfo("🌍"))
				fmt.Println(strings.Repeat("═", 80))
				
				userVars, err := m.List(env.ScopeUser)
				if err != nil {
					return err
				}
				sysVars, err := m.List(env.ScopeSystem)
				if err != nil {
					return err
				}
				
				printVarSection(ui.IconUser, "User", userVars)
				fmt.Println()
				printVarSection(ui.IconSystem, "System", sysVars)
				
				total := len(userVars) + len(sysVars)
				fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("Total: %d variables (User + System)", total)))
				return nil
			}
		},
	}
}

func printVarSection(icon string, title string, vars []env.Variable) {
	fmt.Printf("\n%s %s Environment Variables:\n", icon, ui.HeaderInfo(title))
	fmt.Println(strings.Repeat("─", 80))
	
	// Find max length for alignment
	maxLen := 22 // minimum padding
	for _, v := range vars {
		if len(v.Name) > maxLen {
			maxLen = len(v.Name)
		}
	}
	maxLen += 3 // extra padding
	
	for _, v := range vars {
		value := v.Value
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		padding := strings.Repeat(" ", maxLen-len(v.Name))
		fmt.Printf("  %s%s %s %s\n", ui.KeyValue(v.Name), padding, ui.Dim("="), ui.Path(value))
	}
	
	fmt.Printf("  %s %s\n", ui.Dim(strings.Repeat("─", 78)), ui.Dim(fmt.Sprintf("%s: %d variables", title, len(vars))))
}

func printVarList(m *env.Manager, scopeType env.Scope) error {
	vars, err := m.List(scopeType)
	if err != nil {
		fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
		return err
	}

	icon := ui.IconUser
	title := "User"
	if scopeType == env.ScopeSystem {
		icon = ui.IconSystem
		title = "System"
	}

	printVarSection(icon, title, vars)
	return nil
}

func showSingleScope(m *env.Manager, scopeType env.Scope) error {
	vars, err := m.List(scopeType)
	if err != nil {
		fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
		return err
	}

	fmt.Printf("\n%s %s Environment Variables:\n",
		ui.GetScopeIcon(scope),
		ui.HeaderInfo(scopeType.String()),
	)
	fmt.Println(strings.Repeat("─", 80))

	for _, v := range vars {
		value := v.Value
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		fmt.Printf("  %-30s = %s\n",
			ui.KeyValue(v.Name),
			ui.Path(value),
		)
	}

	fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("Total: %d variables", len(vars))))
	return nil
}

func showBothScopes(m *env.Manager) error {
	fmt.Printf("\n%s Environment Variables - Both Scopes\n", ui.HeaderInfo("🌍"))
	fmt.Println(strings.Repeat("═", 80))

	totalVars := 0

	// Get user variables
	userVars, err := m.List(env.ScopeUser)
	if err != nil {
		fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error reading user variables: %v", err)))
	} else {
		fmt.Printf("\n%s %s Environment Variables:\n",
			ui.IconUser,
			ui.HeaderInfo("User"),
		)
		fmt.Println(strings.Repeat("─", 80))
		for _, v := range userVars {
			value := v.Value
			if len(value) > 60 {
				value = value[:57] + "..."
			}
			fmt.Printf("  %-30s = %s\n",
				ui.KeyValue(v.Name),
				ui.Path(value),
			)
		}
		totalVars += len(userVars)
		fmt.Printf("  %s %s\n", ui.Dim(strings.Repeat("─", 78)), ui.Dim(fmt.Sprintf("User: %d variables", len(userVars))))
	}

	// Get system variables
	sysVars, err := m.List(env.ScopeSystem)
	if err != nil {
		fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error reading system variables: %v", err)))
	} else {
		fmt.Printf("\n%s %s Environment Variables:\n",
			ui.IconSystem,
			ui.HeaderInfo("System"),
		)
		fmt.Println(strings.Repeat("─", 80))
		for _, v := range sysVars {
			value := v.Value
			if len(value) > 60 {
				value = value[:57] + "..."
			}
			fmt.Printf("  %-30s = %s\n",
				ui.KeyValue(v.Name),
				ui.Path(value),
			)
		}
		totalVars += len(sysVars)
		fmt.Printf("  %s %s\n", ui.Dim(strings.Repeat("─", 78)), ui.Dim(fmt.Sprintf("System: %d variables", len(sysVars))))
	}

	fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("Total: %d variables (User + System)", totalVars)))
	return nil
}

func pathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "path",
		Aliases: []string{"p"},
		Short:   fmt.Sprintf("%s Manage PATH entries", ui.IconFolder),
		Long:    "Add, remove, or list entries in the PATH variable",
	}
	
	cmd.AddCommand(
		&cobra.Command{
			Use:     "add [directory]",
			Aliases: []string{"a", "append"},
			Short:   fmt.Sprintf("%s Add directory to PATH", ui.IconPlus),
			Long:    "Add a new directory to the PATH variable",
			Args:    cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := parseScope()
				
				absPath, err := filepath.Abs(args[0])
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error resolving path: %v", err)))
					return err
				}
				
				if _, err := os.Stat(absPath); os.IsNotExist(err) {
					fmt.Printf("%s %s\n", ui.IconWarning, ui.Warning(fmt.Sprintf("Warning: Path does not exist: %s", absPath)))
				}
				
				if err := m.PathAdd("PATH", absPath, scopeType, false); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
				
				fmt.Printf("%s %s %s\n",
					ui.IconSuccess,
					ui.Success("Added to PATH:"),
					ui.Path(absPath),
				)
				
				return nil
			},
		},
		&cobra.Command{
			Use:     "remove [directory]",
			Aliases: []string{"rm", "delete"},
			Short:   fmt.Sprintf("%s Remove directory from PATH", ui.IconMinus),
			Long:    "Remove a directory from the PATH variable",
			Args:    cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := parseScope()
				
				if err := m.PathRemove("PATH", args[0], scopeType); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
				
				fmt.Printf("%s %s %s\n",
					ui.IconSuccess,
					ui.Success("Removed from PATH:"),
					ui.Path(args[0]),
				)
				
				return nil
			},
		},
		&cobra.Command{
			Use:     "list",
			Aliases: []string{"ls", "show"},
			Short:   fmt.Sprintf("%s List PATH entries", ui.IconList),
			Long:    "Display all entries in the PATH variable",
			RunE: func(cmd *cobra.Command, args []string) error {
				m := env.NewManager(dryRun)
				scopeType := parseScope()
				
				entries, err := m.PathList("PATH", scopeType)
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
				
				fmt.Printf("\n%s %s PATH Entries:\n",
					ui.GetScopeIcon(scope),
					ui.HeaderInfo(scopeType.String()),
				)
				fmt.Println(strings.Repeat("─", 80))
				
				for _, entry := range entries {
					status := ui.IconCheck
					if !entry.Exist {
						status = ui.IconBroken
					}
					fmt.Printf("  %s [%3d] %s\n", status, entry.Index, ui.Path(entry.Value))
				}
				
				return nil
			},
		},
	)
	
	return cmd
}

func cleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "clean",
		Aliases: []string{"cleanup", "dedupe"},
		Short:   fmt.Sprintf("%s Clean up PATH variable", ui.IconRefresh),
		Long:    "Remove duplicates and non-existent paths from PATH",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := parseScope()
			
			entries, err := m.PathList("PATH", scopeType)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			
			seen := make(map[string]bool)
			var cleaned []string
			duplicates := 0
			nonexistent := 0
			
			fmt.Printf("\n%s %s PATH Analysis:\n",
				ui.GetScopeIcon(scope),
				ui.HeaderInfo(scopeType.String()),
			)
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
				fmt.Printf("\n%s %s\n", ui.IconInfo, ui.Info(fmt.Sprintf(
					"Would remove %d duplicates and %d non-existent paths",
					duplicates, nonexistent,
				)))
				return nil
			}
			
			if duplicates > 0 || nonexistent > 0 {
				newPath := strings.Join(cleaned, string(os.PathListSeparator))
				if err := m.Set("PATH", newPath, scopeType); err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
					return err
				}
				
				fmt.Printf("\n%s %s\n", ui.IconSuccess, ui.Success(fmt.Sprintf(
					"PATH cleaned: removed %d duplicates and %d non-existent paths",
					duplicates, nonexistent,
				)))
			} else {
				fmt.Printf("\n%s %s\n", ui.IconSuccess, ui.Success("PATH is already clean!"))
			}
			
			return nil
		},
	}
}

func infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info",
		Aliases: []string{"i", "about", "version"},
		Short:   fmt.Sprintf("%s Show environment information", ui.IconInfo),
		Long:    "Display current environment configuration and paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("\n%s PathMan v.1.0.1\n", ui.IconRocket)
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