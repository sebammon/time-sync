# time-sync

A CLI tool that syncs billable time entries from Clockify to YouTrack work items.

## Setup

### 1. Install the binary

#### Option A — Download a release (no Go toolchain needed)

Grab the binary that matches your platform from the [latest release](https://github.com/sebammon/time-sync/releases/latest):

```sh
curl -L -o time-sync https://github.com/sebammon/time-sync/releases/latest/download/time-sync-darwin-arm64
chmod +x time-sync
mv time-sync /opt/homebrew/bin/   # any directory already on your $PATH
```

> **Note:** Avoid `/usr/bin` on macOS — it's protected by System Integrity Protection and isn't writable even with `sudo`. `/opt/homebrew/bin` (Apple Silicon Homebrew) is the cleanest target and is already on `$PATH`.

#### Option B — Build from source

Requires Go 1.22+.

```sh
go install .
```

That puts `time-sync` into `$HOME/go/bin`. Make sure that's on your `$PATH`:

```sh
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

Or build a binary in the current directory without installing:

```sh
go build -o time-sync .
```

### 2. Configure your API credentials

`time-sync` stores credentials in `$XDG_CONFIG_HOME/time-sync/config.json` (defaults to `~/.config/time-sync/config.json`). The file is created with mode `0600`.

Set each key with `time-sync config set`:

```sh
time-sync config set clockify-api-key       <your-clockify-api-key>
time-sync config set clockify-workspace-id  <your-clockify-workspace-id>
time-sync config set clockify-user-id       <your-clockify-user-id>
time-sync config set youtrack-api-key       <your-youtrack-api-key>
```

Inspect what's stored (secrets are masked):

```sh
time-sync config list
```

Read a single value:

```sh
time-sync config get clockify-api-key
```

## Usage

```sh
time-sync [flags]
```

### Flags

| Flag | Description |
| --- | --- |
| `--today` | Sync only today's entries |
| `--yesterday` | Sync only yesterday's entries (default) |
| `--last-n-days <n>` | Sync entries from the last N days (excluding today) |
| `--dry-run` | Print what would be synced without posting anything |
| `--clean-up` | Mark Clockify tasks as done when the linked YouTrack issue is resolved |

### Examples

```sh
# Sync yesterday's entries (default)
time-sync

# Preview what would be synced today
time-sync --today --dry-run

# Sync the last 7 days
time-sync --last-n-days 7

# Clean up completed tasks
time-sync --clean-up
```

## How it works

1. Fetches billable time entries from Clockify for the selected date range
2. Extracts the YouTrack issue ID from each entry's task name (e.g. `PROJ-123`)
3. Checks existing YouTrack work items to avoid duplicates (matched by Clockify entry ID embedded in the description)
4. Posts new work items to the corresponding YouTrack issues, including duration and work type (derived from Clockify tags)

The `--clean-up` mode walks all active Clockify tasks, checks whether the linked YouTrack issue is resolved, and marks resolved ones as done.

## Cutting a release

Releases are built and published locally via [`release.sh`](release.sh). The script builds a `darwin/arm64` binary and creates a GitHub Release with it attached.

Prerequisites: [`gh`](https://cli.github.com/) authenticated against GitHub (`gh auth login`).

```sh
./release.sh v0.1.0
```

The script refuses to run with a dirty working tree, so commit or stash first.
