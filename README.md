# 🗺️ PathMan - Windows Environment Variable Manager

Professional, production-ready CLI tool for managing Windows environment variables with style.

## ✨ Features

- 🎨 **Colorful Output** - Beautiful, intuitive colored output
- 😎 **Emoji Support** - Visual indicators for different states
- 🔐 **Dual Scope** - Manage both User and System variables
- 🛡️ **Safe Mode** - Dry-run to preview changes
- 🧹 **Path Cleanup** - Remove duplicates and invalid paths
- ⚡ **Fast & Lightweight** - Native Windows performance
- 📋 **PATH Management** - Easy add/remove/list operations

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
```

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