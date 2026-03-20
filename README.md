# hive

A CLI tool for managing a structured workspace layout for projects, documents, and code.

Supports both iCloud-backed storage (synced across Macs) and local-only storage.

## Concepts

Projects are organized by **org** (organization/context) and **name**:

For example, orgs might include your personal projects, a client's name, or your employer's name.

Code is optional. Some projects are code-based, others are purely documents. A project can have multiple repositories.

## Filesystem layout

### Top-level directories

```
~/Projects/        — project-shaped things with metadata
~/Life/            — personal documents
~/Work/            — non-project work documents
~/Archive/         — archived materials
~/Code/            — git repos (always local)
```

In iCloud mode, Projects/Life/Work/Archive are symlinked to iCloud Drive. Code is always local.

### Project structure

With code (multi-repo):

```
~/Projects/<org>/<project>/
  code -> ~/Code/<org>/<project>/
  docs/
  assets/
  links/
  .hive.json

~/Code/<org>/<project>/
  <project>-frontend/   (git repo)
  <project>-api/        (git repo)
```

Without code:

```
~/Projects/<org>/<project>/
  docs/
  assets/
  links/
  .hive.json
```

## Install

### From GitHub Releases

```sh
curl -fsSL https://raw.githubusercontent.com/sakakibara/hive/main/scripts/install.sh | sh
```

Installs to `~/.local/bin/hive`. Make sure `~/.local/bin` is in your `PATH`.

### From source

```sh
go install github.com/sakakibara/hive/cmd/hive@latest
```

## Configuration

Config is written automatically by `hive init` to `~/.config/hive/config.toml`:

```toml
[paths]
projects = "~/Projects"
life = "~/Life"
work = "~/Work"
archive = "~/Archive"
code = "~/Code"

[storage]
mode = "icloud" # or "local"
icloud_root = "~/Library/Mobile Documents/com~apple~CloudDocs/workspace"
```

Storage mode is auto-detected during `hive init`. Once written, the mode is never changed automatically.

## Usage

```sh
# Initialize workspace (detects storage mode, writes config, creates dirs)
hive init

# Create a project (no code yet)
hive new <org> <project>

# Clone a repo into a project
hive clone <project> <git-url>

# Create a project and clone in one flow
hive new <org> <project>
hive clone <project> <git-url>

# Adopt an existing repo into a new project
hive adopt <org> <project> <path-to-repo>

# List projects
hive list

# Open a project (for shell use)
cd "$(hive open <project>)"
cd "$(hive open <org>/<project>)"

# Restore code dirs on a new Mac
hive bootstrap

# Fix code symlinks
hive relink

# Check workspace health
hive doctor

# Show storage configuration
hive storage

# Upgrade hive
hive upgrade
```

## Releasing

Releases are automated via GitHub Actions and GoReleaser:

```sh
git tag v0.1.0
git push origin main --tags
```
