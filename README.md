# Developer Worklog CLI

`wl` is a local CLI for tracking developer work sessions by ticket, using a SQLite database and local configuration.

## Prerequisites

- Go 1.22 or later
- macOS or Linux
- Git with offline capture support (`--no-lazy-fetch`) for `wl git`; the implementation has been verified with Git 2.52.0
- An internet connection for downloading Go modules during the first build

## Installation

Clone the repository and build an executable named `wl`:

```sh
git clone https://github.com/taupikpirdian/wlog.git
cd wlog
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/wl" ./cmd/wlog
```

Make sure `~/.local/bin` is on your `PATH`. For the current terminal session:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

To make this permanent, add the line to your shell configuration file, such as `~/.zshrc` or `~/.bashrc`, then open a new terminal.

Verify the installation:

```sh
wl --version
wl --help
```

Local builds display `dev` as the version. Release builds can embed a version number using Go linker flags.

## Usage

Run without arguments to view the active session dashboard and today's tracked time:

```sh
wl
```

On first use, `wl` creates:

```text
~/.worklog/
├── config.yaml
└── worklog.db
```

Subsequent runs reuse the same configuration and database and apply any pending migrations. New directories and files are created with access restricted to the current user.

The dashboard displays the active session's ticket, title, start time, and elapsed duration, followed by today's tracked time per ticket. When there is no work, it displays `No active session` and a total of `0m`. Sessions spanning midnight contribute only the portion that falls within today; the active session's elapsed duration still includes all time since it started.

Review today's events:

```sh
wl today
```

The timeline displays `START`, `NOTE`, `COMMIT`, and `STOP` in chronological order, with a ticket on each line. Dates and times use the device's local time zone, and all repositories in the user's database are included. Active sessions do not receive an artificial `STOP` event.

Notes and commits provide evidence and do not add tracked time. Commits with a ticket but no session appear under `Unsessioned` in the dashboard; commits without a ticket appear under `Unassigned`. The timeline includes both. Daily durations are summed in seconds before being displayed in minutes, so the total may differ from the sum of the displayed minutes for individual tickets.

Both commands read a single consistent snapshot without changing work data, invoking Git, or accessing the network. Multiline notes and commit messages are displayed as a safe single line; their stored contents remain intact. Read failures produce a non-zero exit status rather than an empty view.

View help and version information at any time:

```sh
wl --help
wl help
wl --version
wl version
```

Help and version commands do not create or open the database.

## Tracking Work Sessions

Start a session with a ticket key and an activity title:

```sh
wl start OOT-3751 "Fix tax calculation"
# Alias
wl s OOT-3751 "Fix tax calculation"
```

The ticket is created automatically if it does not exist. The session title does not change the ticket's title. Ticket keys must match `ticket.pattern`, and a session title is required. Configuration and storage are initialized automatically if this is the first command you run.

Only one session can be active across the entire database, including when you switch repositories. If a session is active, `start` displays its details and requests `[Y/n]` confirmation in an interactive terminal. Enter, `y`, or `yes` atomically completes the previous session and starts the new one; `n` or `no` cancels. Piped input and EOF do not provide confirmation.

Complete the active session:

```sh
wl stop
# Alias
wl x
```

Duration is calculated from the start and end times and stored in seconds. Times are displayed in the device's time zone. If the device clock is earlier than the session's start time, the command returns an error and the session remains active. Running `stop` without an active session also returns an error.

When run inside a Git repository, the session stores the repository's root path. Commands also work outside Git or when Git is unavailable.

## Recording Missed Time

Add a completed session with a time range you specify:

```sh
wl session OOT-3751 --from 09:00 --to 11:00 --title "Fix tax calculation"
```

Or start an active session from an earlier time:

```sh
wl s OOT-3751 "Fix tax" --since 09:00
# Shorthand flag
wl s OOT-3751 "Fix tax" -s 09:00
```

Times must use the exact `HH:mm` format on today's local date. Future times, empty or reversed completed ranges, and ambiguous or nonexistent local times caused by time zone offset changes are rejected. There is no automatic rollover to yesterday or across midnight; duration is calculated from actual elapsed time without AI estimation.

Both operations reject overlaps with sessions on any ticket or repository. Ranges that only touch at their boundaries are allowed. `--since` requires no active session and does not offer to switch sessions; starting without this flag retains the existing confirmation behavior. A completed manual session can be added before an active session's start time without changing that active session.

New sessions appear immediately in the dashboard and timeline. Existing notes and commits are not moved into the new session, even if their times and tickets match. Tickets and sessions are stored atomically; a successful command creates one session. If output fails after storage succeeds, review `wl` or `wl today` before retrying because the session may already be stored and a retry will be rejected as a conflict.

## Adding Activity Notes

While a session is active, record investigation context or work beyond coding:

```sh
wl n "Check Splunk logs"
wl note "Found response mismatch"
wl n "Support QA retest"
```

Each command stores one `NOTE` activity on the active ticket and session, then displays `✓ Note added to <ticket>`. Text must be provided as a single argument; leading and trailing whitespace is trimmed, and empty descriptions are rejected. Notes with identical text may be added multiple times.

Notes work outside Git and inherit the active session's repository context. Adding a note does not change the session's duration. Without an active session, the command fails and suggests `wl s <ticket> "<title>"`. If the session changes during storage, the command fails; retry to use the latest session.

## Capturing Git Commits

Inside a repository that already has a commit, run:

```sh
wl git
```

The command captures the latest HEAD along with its message, branch when available, changed files, text line statistics, and commit time. The ticket is selected in this order: commit message, active session, branch, then `UNASSIGNED`. If the ticket matches the active session, the commit is linked to that session. Otherwise, it is stored under its ticket without a session and a warning is displayed.

Commits can be captured without an active session and do not add tracked time. Capturing the same repository and hash again displays `✓ Commit already captured.` without duplicating or moving the existing record.

`capture_changed_files` and `capture_diff_stat` can be disabled independently. Statistics exclude `.env*` contents; binary files do not contribute to text line counts. Full diffs are not collected, even when `capture_full_diff: true` is configured—the command displays a warning and still stores metadata. Unavailable supplementary metadata produces warnings; Git identity or storage failures produce a non-zero exit status.

## Installing Automatic Git Hooks

Run from the root or a subdirectory of a Git repository:

```sh
wl install-hooks
# Alias
wl install-hook
```

The installer places a `post-commit` hook in Git's default hook directory, including for repositories with no commits yet. Output shows the repository and hook path. Installation does not create configuration or a database, or perform a capture. On subsequent commits, the wrapper runs:

```sh
wl git >/dev/null 2>&1 || true
```

Make sure `wl` is on the Git process's `PATH`, including when committing from an editor or GUI. Capture output is discarded; capture failures or a missing executable do not cancel the commit. Manual capture with `wl git` remains available.

An existing hook that is a regular file is copied intact to the sibling file `post-commit.wlog-original`, preserving its read, write, and execute permissions. An executable original hook runs before capture, retaining its output and exit status. A nonexecutable original is preserved without being run. Because the original runs from its backup filename, hooks that depend on `$0` or their basename require manual integration.

Reinstalling a valid wrapper displays `already installed` without duplicating capture. Missing executable permissions are repaired without changing the wrapper's contents. Default hooks for linked worktrees are shared across the repository's worktrees; this scope is shown in the output.

A configured `core.hooksPath`, symlink or nonregular hook, reserved backup that conflicts with a new installation, or modified managed wrapper or backup is rejected with an error. For a custom hook manager or path, manually add the capture command above to the hook you manage. The installer does not change Git configuration.

Installation uses a lock and atomic publication. Stale locks or recovery files left after a crash require manual inspection using the paths shown in the error. The lock coordinates `wl` installers; detected changes by another editor abort installation, but that editor may still race with installation after the final check.

## Running from Source

To run directly without installing an executable:

```sh
go run ./cmd/wlog
```

Or build in the repository directory:

```sh
go build -o ./wl ./cmd/wlog
./wl
```

## Configuration

The configuration file is created automatically at `~/.worklog/config.yaml`:

```yaml
database:
  path: ~/.worklog/worklog.db

ticket:
  pattern: "[A-Z][A-Z0-9]+-[0-9]+"

git:
  capture_changed_files: true
  capture_diff_stat: true
  capture_full_diff: false

ai:
  enabled: true
  include_diff: false
```

Existing values are preserved when `wl` is run again. `capture_full_diff` and `include_diff` are disabled by default.

## Unit Tests and Coverage

Every package with executable code in `application` and `domain` has unit tests with **100% statement coverage**. Root packages containing only `doc.go` have no measurable statements.

Run unit tests for both layers and generate a coverage report:

```sh
go test -race -coverprofile=coverage.out ./application/... ./domain/...
go tool cover -func=coverage.out
go tool cover -html=coverage.out
```

Run regression tests across the project:

```sh
go test -race ./...
go vet ./...
```

## Feature Status

- Available: CLI and local storage initialization, ticket management in the application and storage layers, session start and stop, activity notes, Git commit capture, Git hook installation, dashboard, today's timeline, manual completed sessions, and backdated starts.
- Summary, description, session editing or deletion, and manual ranges for historical dates are not yet available.
