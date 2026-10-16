/*
 FILE: internal/cli/safety.go

 Safety layer:
   - newManager(): every registry write is preceded by an automatic,
     verified backup of the affected scope. If the backup cannot be written
     the change is NOT made.
   - `pathman config`  show / path / init
   - `pathman backup`  list / dir / create / restore

 Global flags:  --config FILE   --backup-dir DIR   --no-backup
*/

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/registry"

	"github.com/cumulus13/pathman/internal/backup"
	"github.com/cumulus13/pathman/internal/config"
	"github.com/cumulus13/pathman/internal/env"
	"github.com/cumulus13/pathman/internal/ui"
)

var (
	cfgFlag       string
	backupDirFlag string
	noBackupFlag  bool

	settings      *config.Settings
	currentAction = "pathman"
)

const iconBackup = "💾"

// getSettings resolves configuration once. It is lazy on purpose: read-only
// commands keep working even if the config file is broken, but any write is
// refused until the config is fixed.
func getSettings() (*config.Settings, error) {
	if settings != nil {
		return settings, nil
	}
	s, err := config.Resolve(config.Overrides{
		ConfigPath: cfgFlag,
		BackupDir:  backupDirFlag,
		NoBackup:   noBackupFlag,
	})
	if err != nil {
		return nil, err
	}
	settings = s
	return s, nil
}

// newManager creates a Manager whose writes are protected by auto-backup.
// Use this instead of env.NewManager everywhere in the CLI.
func newManager() *env.Manager {
	m := env.NewManager(dryRun)
	if dryRun {
		return m // nothing is written in dry-run mode
	}
	done := map[env.Scope]bool{} // one backup per scope per command run
	m.BeforeWrite = func(sc env.Scope) error {
		s, err := getSettings()
		if err != nil {
			return err
		}
		if !s.BackupEnabled || done[sc] {
			return nil
		}
		path, err := takeBackup(m, sc, currentAction, os.Args[1:], s)
		if err != nil {
			return err
		}
		done[sc] = true
		fmt.Fprintf(os.Stderr, "%s %s %s\n", iconBackup, ui.Dim("Backup saved:"), ui.Path(path))
		if n, perr := backup.Prune(s.BackupDir, s.BackupKeep); perr != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", ui.IconWarning, ui.Warning("backup prune: "+perr.Error()))
		} else if n > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", ui.Dim(fmt.Sprintf("   (removed %d old backups, keeping %d)", n, s.BackupKeep)))
		}
		return nil
	}
	return m
}

func scopeName(sc env.Scope) string {
	if sc == env.ScopeSystem {
		return "system"
	}
	return "user"
}

func regTypeName(t uint32) string {
	switch t {
	case registry.SZ:
		return "REG_SZ"
	case registry.EXPAND_SZ:
		return "REG_EXPAND_SZ"
	default:
		return fmt.Sprintf("REG_%d", t)
	}
}

func takeBackup(m *env.Manager, sc env.Scope, action string, args []string, s *config.Settings) (string, error) {
	raw, err := m.Snapshot(sc)
	if err != nil {
		return "", fmt.Errorf("cannot read current %s environment: %w", scopeName(sc), err)
	}
	vals := make([]backup.Value, 0, len(raw))
	for _, r := range raw {
		v := backup.Value{Name: r.Name, Type: regTypeName(r.Type)}
		if r.Type == registry.SZ || r.Type == registry.EXPAND_SZ {
			v.Value = r.String
		} else {
			v.Raw = r.Data
		}
		vals = append(vals, v)
	}
	return backup.Write(s.BackupDir, backup.New(scopeName(sc), action, args, vals))
}

// ─── config ─────────────────────────────────────────────────────────────────

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: fmt.Sprintf("%s Show or create the pathman config file", ui.IconGear),
		Long: `Manage pathman's config file.

The config file is never looked up in the current directory. Order:
  1. --config FILE
  2. $PATHMAN_CONFIG
  3. <folder of pathman.exe>\pathman.json
  4. %AppData%\pathman\config.json
  5. built-in defaults

Example config:
  {
    "backup": {
      "enabled": true,
      "dir": "D:\\Backups\\pathman",
      "keep": 100
    }
  }

"dir" accepts %VAR% / $VAR / ~ ; a relative dir is relative to the config
file's folder. "keep" = number of backups to retain (0 = keep all).
Backup dir precedence: --backup-dir > $PATHMAN_BACKUP_DIR > config > default.`,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show the effective settings and where each value comes from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getSettings()
			if err != nil {
				return err
			}
			file := s.ConfigFile
			if file == "" {
				file = "(none — using built-in defaults)"
			}
			state := "enabled"
			if !s.BackupEnabled {
				state = "DISABLED"
			}
			keep := fmt.Sprint(s.BackupKeep)
			if s.BackupKeep == 0 {
				keep = "all (no pruning)"
			}
			fmt.Printf("%s %s\n", ui.Dim("Config file :"), ui.Path(file))
			fmt.Printf("%s %s %s\n", ui.Dim("Backup       :"), state, ui.Dim("("+s.EnabledSource+")"))
			fmt.Printf("%s %s %s\n", ui.Dim("Backup dir   :"), ui.Path(s.BackupDir), ui.Dim("("+s.DirSource+")"))
			fmt.Printf("%s %s %s\n", ui.Dim("Keep         :"), keep, ui.Dim("("+s.KeepSource+")"))
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the config file in use and the locations that are searched",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getSettings()
			if err != nil {
				return err
			}
			if s.ConfigFile != "" {
				fmt.Println(s.ConfigFile)
			} else {
				fmt.Println("(no config file found — using defaults)")
			}
			if def, err := config.DefaultConfigPath(); err == nil {
				fmt.Fprintf(os.Stderr, "%s %s\n", ui.Dim("default location:"), def)
			}
			if exe, err := os.Executable(); err == nil {
				fmt.Fprintf(os.Stderr, "%s %s\n", ui.Dim("portable location:"), filepath.Join(filepath.Dir(exe), "pathman.json"))
			}
			return nil
		},
	})

	{
		var force bool
		initCmd := &cobra.Command{
			Use:   "init",
			Short: "Write a default config file (to --config FILE, or the default location)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				target := cfgFlag
				if target == "" {
					p, err := config.DefaultConfigPath()
					if err != nil {
						return err
					}
					target = p
				}
				abs, err := filepath.Abs(target)
				if err != nil {
					return err
				}
				if _, err := os.Stat(abs); err == nil && !force {
					return fmt.Errorf("%s already exists (use --force to overwrite)", abs)
				}
				tpl, err := config.Template()
				if err != nil {
					return err
				}
				if dryRun {
					fmt.Printf("%s Would write %s:\n%s", ui.IconInfo, ui.Path(abs), tpl)
					return nil
				}
				if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(abs, []byte(tpl), 0o600); err != nil {
					return err
				}
				fmt.Printf("%s Config written: %s\n", ui.IconSuccess, ui.Path(abs))
				return nil
			},
		}
		initCmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing config file")
		cmd.AddCommand(initCmd)
	}
	return cmd
}

// ─── backup ─────────────────────────────────────────────────────────────────

func backupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: fmt.Sprintf("%s List, create and restore environment backups", iconBackup),
		Long: `Pathman automatically saves a snapshot of the affected scope (user or system)
BEFORE every change (set, delete, append, edit, remove, clean, ...). If the
snapshot cannot be written, the change is not made.

Snapshots keep the registry value type, so REG_EXPAND_SZ entries such as
%SystemRoot%\system32 survive a restore.

Disable for one run with --no-backup, or permanently via the config file.
Change the folder with --backup-dir, $PATHMAN_BACKUP_DIR or the config file.`,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "dir",
		Short: "Print the backup directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getSettings()
			if err != nil {
				return err
			}
			fmt.Println(s.BackupDir)
			return nil
		},
	})

	{
		var scope string
		listCmd := &cobra.Command{
			Use:     "list",
			Aliases: []string{"ls"},
			Short:   "List backups (newest first)",
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				s, err := getSettings()
				if err != nil {
					return err
				}
				filter := ""
				if scope != "" && scope != "both" {
					filter = scopeName(scopeFromString(scope))
				}
				es, err := backup.List(s.BackupDir, filter)
				if err != nil {
					return err
				}
				if len(es) == 0 {
					fmt.Printf("%s No backups in %s\n", ui.IconInfo, ui.Path(s.BackupDir))
					return nil
				}
				fmt.Printf("%s\n\n", ui.Dim(s.BackupDir))
				for _, e := range es {
					fmt.Printf("%s  %-6s  %3d vars  %s\n      %s\n",
						filepath.Base(e.Path), e.Snapshot.Scope, len(e.Snapshot.Values),
						ui.Dim(e.Snapshot.Action), ui.Dim(strings.Join(e.Snapshot.Args, " ")))
				}
				return nil
			},
		}
		listCmd.Flags().StringVarP(&scope, "scope", "s", "both", "Scope: user, system or both")
		cmd.AddCommand(listCmd)
	}

	{
		var scope string
		createCmd := &cobra.Command{
			Use:   "create",
			Short: "Take a backup now (without changing anything)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				s, err := getSettings()
				if err != nil {
					return err
				}
				scopes := []env.Scope{env.ScopeUser, env.ScopeSystem}
				if scope != "" && scope != "both" {
					scopes = []env.Scope{scopeFromString(scope)}
				}
				m := env.NewManager(false)
				for _, sc := range scopes {
					if dryRun {
						fmt.Printf("%s Would back up %s scope to %s\n", ui.IconInfo, scopeName(sc), ui.Path(s.BackupDir))
						continue
					}
					p, err := takeBackup(m, sc, "pathman backup create", os.Args[1:], s)
					if err != nil {
						return err
					}
					fmt.Printf("%s %s %s\n", ui.IconSuccess, scopeName(sc), ui.Path(p))
				}
				return nil
			},
		}
		createCmd.Flags().StringVarP(&scope, "scope", "s", "both", "Scope: user, system or both")
		cmd.AddCommand(createCmd)
	}

	{
		var vars []string
		var confirm, refresh bool
		restoreCmd := &cobra.Command{
			Use:   "restore <latest|file>",
			Short: "Restore variables from a backup",
			Long: `Restore variables from a backup file ("latest" = newest, or a file name / path).

The backup's own scope is used (a user backup is never applied to system).
Only variables that differ are touched; variables that do not exist in the
backup are left alone. Without --confirm the differences are only listed.
A fresh backup of the current state is taken before restoring, so a restore
can itself be undone.

Examples:
  pathman backup restore latest
  pathman backup restore latest --var PATH --confirm
  pathman backup restore 20261016T074500.123Z_system.json --confirm`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				s, err := getSettings()
				if err != nil {
					return err
				}
				path, err := backup.Resolve(s.BackupDir, args[0], "")
				if err != nil {
					return err
				}
				snap, err := backup.Read(path)
				if err != nil {
					return err
				}
				sc := env.ScopeUser
				if snap.Scope == "system" {
					sc = env.ScopeSystem
				}

				m := newManager()
				cur, err := m.Snapshot(sc)
				if err != nil {
					return err
				}
				curBy := map[string]env.RawValue{}
				for _, c := range cur {
					curBy[strings.ToLower(c.Name)] = c
				}

				want := map[string]bool{}
				for _, v := range vars {
					want[strings.ToLower(v)] = true
				}

				type change struct {
					v      backup.Value
					create bool
				}
				var todo []change
				skipped := 0
				matched := 0
				for _, v := range snap.Values {
					if len(want) > 0 && !want[strings.ToLower(v.Name)] {
						continue
					}
					matched++
					if !v.IsString() {
						skipped++
						continue
					}
					c, ok := curBy[strings.ToLower(v.Name)]
					wantType := uint32(registry.SZ)
					if v.Type == "REG_EXPAND_SZ" {
						wantType = registry.EXPAND_SZ
					}
					if ok && c.Type == wantType && c.String == v.Value {
						continue
					}
					todo = append(todo, change{v, !ok})
				}
				if len(want) > 0 && matched == 0 {
					return fmt.Errorf("none of the requested variables exist in %s", filepath.Base(path))
				}

				fmt.Printf("%s %s %s (%s scope, %s)\n", iconBackup, ui.Dim("Backup:"), ui.Path(filepath.Base(path)), snap.Scope, snap.Created)
				if len(todo) == 0 {
					fmt.Printf("%s Nothing to restore — current values already match.\n", ui.IconCheck)
					return nil
				}
				for _, c := range todo {
					verb := "restore"
					if c.create {
						verb = "re-create"
					}
					fmt.Printf("  %s %s\n", verb, ui.Highlight(c.v.Name))
				}
				if skipped > 0 {
					fmt.Printf("%s %d non-string value(s) skipped (pathman does not manage those)\n", ui.IconInfo, skipped)
				}

				if dryRun {
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("Dry-run: nothing changed"))
					return nil
				}
				if !confirm {
					return fmt.Errorf("%d variable(s) would change; re-run with --confirm to apply", len(todo))
				}

				for _, c := range todo {
					if err := m.RestoreValue(c.v.Name, c.v.Value, c.v.Type == "REG_EXPAND_SZ", sc); err != nil {
						return err
					}
					fmt.Printf("%s Restored %s\n", ui.IconSuccess, ui.Highlight(c.v.Name))
				}
				if refresh {
					m.BroadcastChange()
					fmt.Printf("%s %s\n", ui.IconInfo, ui.Info("[WM_SETTINGCHANGE broadcast]"))
				}
				return nil
			},
		}
		restoreCmd.Flags().StringSliceVar(&vars, "var", nil, "Restore only this variable (repeatable)")
		restoreCmd.Flags().BoolVar(&confirm, "confirm", false, "Apply the restore (otherwise only list differences)")
		restoreCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Broadcast WM_SETTINGCHANGE after restoring")
		cmd.AddCommand(restoreCmd)
	}
	return cmd
}
