# 🗺️ PathMan - Windows Environment Variable Manager

[![CI](https://github.com/cumulus13/pathman/actions/workflows/ci.yml/badge.svg)](https://github.com/cumulus13/pathman/actions/workflows/ci.yml)
[![CodeQL](https://github.com/cumulus13/pathman/actions/workflows/codeql.yml/badge.svg)](https://github.com/cumulus13/pathman/actions/workflows/codeql.yml)
[![Release](https://img.shields.io/github/v/release/cumulus13/pathman)](https://github.com/cumulus13/pathman/releases)

Professional, production-ready CLI tool for managing Windows environment variables with style.

## ✨ Features

- 🎨 **Colorful Output** - Beautiful, intuitive colored output
- 😎 **Emoji Support** - Visual indicators for different states
- 🔐 **Dual Scope** - Manage both User and System variables
- 🛡️ **Safe Mode** - Dry-run to preview changes
- 🧹 **Path Cleanup** - Remove duplicates and invalid paths
- ⚡ **Fast & Lightweight** - Native Windows performance
- 📋 **PATH Management** - Easy add/remove/list operations
- 🔍 **Find** - Search names, values and PATH entries across all variables or just one

## 🚀 Installation

### Using Go
```bash
go install github.com/cumulus13/pathman/cmd/pathman@latest
```

### Manual Download
Download the latest release from [Releases](https://github.com/cumulus13/pathman/releases)

## 📖 Usage

### Basic Commands

```bash
# Get a variable value
pathman get PATH
pathman get -s system JAVA_HOME

# Set a variable
pathman set MY_VAR "C:\my\path"
pathman set -s system JAVA_HOME "C:\Program Files\Java\jdk-17"

# Delete a variable
pathman delete MY_VAR
pathman delete -s system OLD_VAR

# List all variables
pathman list
pathman list -s system

# Show system info
pathman info
```

### PATH Management

```bash
# Add to PATH
pathman path add "C:\MyTools"
pathman path add -s system "C:\Program Files\MyApp"

# Remove from PATH
pathman path remove "C:\MyTools"
pathman path remove -s system "C:\OldApp"

# List PATH entries
pathman path list
pathman path list -s system

# Clean up PATH (remove duplicates and invalid paths)
pathman clean
pathman clean -s system --dry-run  # Preview changes

# Get single variable as JSON
pathman get PATH --format json
pathman get JAVA_HOME -f json

# Get from specific scope as JSON
pathman get PATH --scope user --format json
pathman get PATH --scope system -f json

# Get as YAML
pathman get PATH -f yaml

# Get as CSV
pathman get PATH -f csv

# List all variables as JSON
pathman list -f json

# List system only as JSON
pathman list --scope system -f json

# Export to file
pathman list -f json > env_vars.json
pathman list -f csv > env_vars.csv
pathman get PATH -f json > path.json

# Pretty print with jq (if installed)
pathman list -f json | jq .
```

### Automatic backup & restore

Before **every** change (`set`, `delete`, `append`, `edit`, `remove-value`, `path add/remove`, `clean`, `backup restore`, ...) pathman saves a verified snapshot of the affected scope (user or system). If the snapshot cannot be written, **the change is not made**. Snapshots keep the registry value type, so `REG_EXPAND_SZ` entries like `%SystemRoot%\system32` survive.

```
pathman backup list                     # newest first
pathman backup create -s system         # manual snapshot
pathman backup restore latest           # show what would change
pathman backup restore latest --confirm # apply
pathman backup restore latest --var PATH --confirm
pathman backup dir                      # where backups live

pathman edit PATH Foo Bar --no-backup   # skip the backup for one run
pathman set X 1 --backup-dir D:\bk      # use another folder for one run
```

Default backup folder: Windows `%LOCALAPPDATA%\pathman\backups`, macOS `~/Library/Application Support/pathman/backups`, Linux `$XDG_STATE_HOME` or `~/.local/state/pathman/backups`.
Backups contain your variable values - keep the folder private.

### Config file

```
pathman config init      # write a default config
pathman config show      # effective settings + where each value came from
pathman config path
```

The config file is **never** read from the current directory, so results do not depend on where you run pathman. Lookup order: `--config FILE` -> `$PATHMAN_CONFIG` -> `<folder of pathman.exe>\pathman.json` -> `%AppData%\pathman\config.json` -> built-in defaults. A broken config is an error (never silently ignored) and blocks changes until fixed; unknown keys are rejected.

```json
{
  "backup": {
    "enabled": true,
    "dir": "D:\\Backups\\pathman",
    "keep": 100
  }
}
```

`dir` accepts `%VAR%`, `$VAR`, `~`; a relative `dir` is relative to the config file's folder. `keep` = backups retained (`0` = keep all; only pathman's own snapshot files are ever pruned).
Backup folder precedence: `--backup-dir` > `$PATHMAN_BACKUP_DIR` > config `backup.dir` > OS default.

### Edit entries inside a variable

Works on any `;`-separated variable (PATH, PATHEXT, PYTHONPATH, ...), keeps entry order.

```
pathman edit PATH "C:\SDK\lua" "C:\SDK\lua\55"      # whole-entry match (won't touch C:\SDK\luarocks)
pathman edit PATH Python38 Python312 --contains    # substring replace
pathman edit PATH "C:\Python3*" "C:\Py312" --confirm # glob: whole entry replaced
pathman edit PATH "^C:\\Old(.*)" "D:\New$1" -R        # regex with groups
pathman edit PATH --at 4 "C:\Tools\bin" -s system     # replace entry #4 (numbers from `find`)
pathman edit PYTHONPATH "C:\lib" "D:\lib" --dry-run   # preview

pathman remove-value PATH "C:\Old*" --confirm    # delete entries (any variable)
pathman append PATH "C:\New" -p pre              # add entry (front/end)
```

### Find

```
# Search EVERYTHING (names, values, PATH-style entries) in user + system scope
pathman find python

# Search inside ONE variable only
pathman find python -e PATH
pathman find "C:\Program Files*" -e PATH -s system

# Restrict what is searched: name | value | entry
pathman find java --in name
pathman find "C:\Go*" --in value

# Glob / regex
pathman find "C:\Python3*"
pathman find "^C:\\Go" -R -e PATH

# Flag PATH entries whose directory no longer exists
pathman find . -e PATH --exists

# Only names of matching variables / machine-readable output
pathman find mingw -l
pathman find python -f json
```

Query: plain text = case-insensitive substring; `* ? [abc]` = glob (whole text); `-R` = regex.
Flags: `-e/--env NAME`, `--in all|name|value|entry`, `-s/--scope user|system|both` (default `both`), `-R/--regex`, `--exists`, `-l/--list`.

### Flags

- `-s, --scope` - Set scope (user|system) [default: user]
- `--dry-run` - Show what would be done without making changes
- `--no-color` - Disable colored output
- `-h, --help` - Show help
- `-v, --version` - Show version

### Aliases

Most commands have short aliases:
- `get` → `g`, `show`, `value`
- `set` → `s`, `add`
- `delete` → `d`, `rm`, `remove`, `unset`
- `list` → `l`, `ls`, `all`
- `path` → `p`
- `path add` → `a`, `append`
- `path list` → `ls`, `show`
- `clean` → `cleanup`, `dedupe`
- `edit` → `replace`, `change`, `sub`
- `find` → `search`, `grep`, `fd`, `locate`

## 🔒 Permissions

- **User scope**: No special permissions required- **System scope**: Requires Administrator privileges

## 🛠️ Development

### Prerequisites
- Go 1.21+
- Windows OS
- Administrator access (for system scope testing)

### Build
```bash
make build    # Build for current platform
make build-all # Cross-compile
make test     # Run tests
make lint     # Run linter
make install  # Install to GOPATH
```

## 📝 License

MIT License - see [LICENSE](LICENSE) for details.

## 🤝 Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

## 👤 Author
        
[Hadi Cahyadi](mailto:cumulus13@gmail.com)
    

[![Buy Me a Coffee](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/cumulus13)

[![Donate via Ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/cumulus13)
 
[Support me on Patreon](https://www.patreon.com/cumulus13)

## 📦 Installer (Windows)

Each release ships `pathman_<version>_windows_<arch>_setup.exe` for `amd64`, `386` and `arm64` (plus the plain `.zip`).

```
pathman_1.0.5_windows_amd64_setup.exe            # interactive
pathman_1.0.5_windows_amd64_setup.exe /S         # silent
pathman_1.0.5_windows_amd64_setup.exe /S /D=C:\Tools\PathMan   # silent, custom folder (/D= must be last, no quotes)
"C:\Program Files\PathMan\uninstall.exe" /S     # silent uninstall
```

- Installs to `C:\Program Files\PathMan` (needs administrator), registers in *Apps & features*, optional **Add to system PATH** component (on by default).
- PATH is changed by `pathman.exe path add/remove` itself, not by the installer script: no length limit, registry type (`REG_EXPAND_SZ`) is preserved, and an automatic backup of the system environment is taken first (`pathman backup list`).
- Uninstall removes only its own PATH entry and files; your backups are kept.
- Install folders containing `;  [  ]` are refused (they would corrupt or mis-match PATH entries).
- Build it yourself: `makensis -DVERSION=1.0.5 -DVERSION_NUM=1.0.5.0 -DARCH=amd64 -DEXE=<abs path to pathman.exe> -DSRCDIR=<abs repo root> -DOUTFILE=<abs out.exe> installer/pathman.nsi`

> **Debian / Alpine:** PathMan manages the Windows registry and does not build for Linux, so there are no `.deb` / `.apk` packages.

## 🤖 CI / Releases

| Workflow | When | What |
|---|---|---|
| `ci.yml` | push / PR | portable tests (race), cross-compile win amd64/386/arm64, `go vet`, full tests on Windows, **end-to-end run against the real registry** (`.github/scripts/e2e.ps1`), installer build + real install/upgrade/uninstall (`.github/scripts/installer-e2e.ps1`) |
| `release.yml` | tag `v*` | tests, then GoReleaser builds zips + **NSIS installers** (amd64/386/arm64) + `checksums.txt` and publishes a GitHub Release. Manual run = snapshot build only |
| `codeql.yml` | push / PR / weekly | CodeQL analysis |
| `security.yml` | `go.mod` change / weekly | `govulncheck` |
| `dependabot.yml` | weekly | Go modules + GitHub Actions updates |

Release: `git tag v1.0.5 && git push origin v1.0.5`
