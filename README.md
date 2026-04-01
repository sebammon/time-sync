# time-sync

A CLI tool that syncs billable time entries from Clockify to YouTrack work items.

## Setup

1. Install dependencies:

```sh
pnpm install
```

2. Create a `.env` file in the project root with the following variables:

```
CLOCKIFY_API_KEY=<your-clockify-api-key>
YOUTRACK_API_KEY=<your-youtrack-api-key>
CLOCKIFY_WORKSPACE_ID=<your-clockify-workspace-id>
CLOCKIFY_USER_ID=<your-clockify-user-id>
```

## Usage

```sh
pnpm start [options]
```

### Options

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
pnpm start

# Preview what would be synced today
pnpm start --today --dry-run

# Sync the last 7 days
pnpm start --last-n-days 7

# Clean up completed tasks
pnpm start --clean-up
```

## Installing as a global command

To make `time-sync` available as a command anywhere on your system:

```sh
pnpm link --global
```

Then run it directly:

```sh
time-sync --today --dry-run
```

To uninstall the global link:

```sh
pnpm unlink --global time-sync
```

## How it works

1. Fetches billable time entries from Clockify for the selected date range
2. Extracts the YouTrack issue ID from each entry's task name (e.g. `PROJ-123`)
3. Checks existing YouTrack work items to avoid duplicates (matched by Clockify entry ID embedded in the description)
4. Posts new work items to the corresponding YouTrack issues, including duration and work type (derived from Clockify tags)

The `--clean-up` mode walks all active Clockify tasks, checks whether the linked YouTrack issue is resolved, and marks resolved ones as done.
