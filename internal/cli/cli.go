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
		Long: fmt.Sprintf(`%s
PathMan - A modern, professional Windows environment variable manager

%s Manage PATH and other environment variables with ease.
Supports both User and System scopes with colorful, intuitive output.

%s Features:
  • Add, remove, and list PATH entries
  • Set, get, and delete environment variables
  • User and System scope management
  • Duplicate detection and cleanup
  • Non-existent path validation
  • Dry-run mode for safe testing`,
			ui.HeaderInfo("🌍 PathMan - Environment Variable Manager"),
			ui.Info,
			ui.Dim,
		),
		Version: "1.0.0",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				color.NoColor = true
			}
		},
	}
	
	rootCmd.PersistentFlags().StringVarP(&scope, "scope", "s", "user", "Scope: user or system")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	
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
	default:
		return env.ScopeUser
	}
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get [variable]",
		Aliases: []string{"g", "show", "value"},
		Short:   fmt.Sprintf("%s Get environment variable value", ui.IconSearch),
		Long:    "Retrieve and display the value of an environment variable",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := parseScope()
			
			v, err := m.Get(args[0], scopeType)
			if err != nil {
				fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error: %v", err)))
				return err
			}
			
			fmt.Printf("\n%s %s\n", ui.GetScopeIcon(scope), ui.Highlight(v.Name))
			fmt.Printf("%s %s\n", ui.IconLink, ui.Path(v.Value))
			
			return nil
		},
	}
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

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls", "all"},
		Short:   fmt.Sprintf("%s List all environment variables", ui.IconList),
		Long:    "Display all environment variables in the specified scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := env.NewManager(dryRun)
			scopeType := parseScope()
			
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
				fmt.Printf("  %s %s %s\n",
					ui.KeyValue(v.Name),
					ui.Dim("="),
					ui.Path(value),
				)
			}
			
			fmt.Printf("\n%s %s\n", ui.IconCheck, ui.Dim(fmt.Sprintf("Total: %d variables", len(vars))))
			
			return nil
		},
	}
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
				
				// Resolve to absolute path
				absPath, err := filepath.Abs(args[0])
				if err != nil {
					fmt.Printf("%s %s\n", ui.IconError, ui.Error(fmt.Sprintf("Error resolving path: %v", err)))
					return err
				}
				
				// Check if path exists
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
			fmt.Printf("\n%s PathMan v1.0.0\n", ui.IconRocket)
			fmt.Println(strings.Repeat("═", 60))
			
			fmt.Printf("\n%s %s\n", ui.IconGear, ui.HeaderInfo("System Info"))
			fmt.Printf("  %s %s\n", ui.IconUser, ui.Info("User:"), os.Getenv("USERNAME"))
			fmt.Printf("  %s %s\n", ui.IconSystem, ui.Info("Computer:"), os.Getenv("COMPUTERNAME"))
			fmt.Printf("  %s %s\n", ui.IconFolder, ui.Info("Home:"), os.Getenv("USERPROFILE"))
			fmt.Printf("  %s %s\n", ui.IconGear, ui.Info("OS:"), os.Getenv("OS"))
			
			fmt.Printf("\n%s %s\n", ui.IconStar, ui.HeaderInfo("Important Paths"))
			fmt.Printf("  %s %s\n", ui.IconFolder, ui.Info("System Root:"), os.Getenv("SystemRoot"))
			fmt.Printf("  %s %s\n", ui.IconFolder, ui.Info("Program Files:"), os.Getenv("ProgramFiles"))
			fmt.Printf("  %s %s\n", ui.IconFolder, ui.Info("AppData:"), os.Getenv("APPDATA"))
			fmt.Printf("  %s %s\n", ui.IconFolder, ui.Info("Temp:"), os.Getenv("TEMP"))
			
			fmt.Printf("\n%s %s\n", ui.IconInfo, ui.Dim("Use 'pathman --help' for available commands"))
			
			return nil
		},
	}
}