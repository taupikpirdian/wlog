# Quickstart Validation: Initialize CLI and Local Storage

## Prerequisites

- Go toolchain compatible with `go.mod`.
- A writable temporary home directory for isolated first-run checks.
- No network access is needed for the scenarios below.

## Build

From the repository root:

```sh
go build -o ./wl ./cmd/wlog
```

## Scenario 1 — Help and version without local storage

Use an empty temporary home directory and run:

```sh
HOME="$(mktemp -d)" ./wl --help
HOME="$(mktemp -d)" ./wl --version
```

Expected: both commands exit with status 0 and show usage/version. Neither invocation requires SQLite or creates `~/.worklog`.

## Scenario 2 — First-run initialization

```sh
TEST_HOME="$(mktemp -d)"
HOME="$TEST_HOME" ./wl
find "$TEST_HOME/.worklog" -maxdepth 1 -type f -print
```

Expected: `config.yaml` and `worklog.db` exist. The configuration contains the defaults in `contracts/cli.md`; the database is migrated to schema version 1 and includes the entities and indexes in `data-model.md`.

## Scenario 3 — Idempotent repeated startup

Change one config value, run `wl` again, and inspect the config/database:

```sh
HOME="$TEST_HOME" ./wl
```

Expected: the custom value remains, existing rows remain, and the schema version stays at the latest version.

## Scenario 4 — Invalid configuration

Replace the config content with invalid YAML and run `wl`.

Expected: non-zero exit, a clear message naming the config path, no silent replacement of the file, and no credential disclosure.

## Scenario 5 — Unwritable home/data path

Run `wl` with a home directory that cannot be written by the current user.

Expected: non-zero exit with an actionable path-specific error. `wl --help` and `wl --version` still succeed.

## Scenario 6 — Migration rollback and newer schema

In an isolated temporary database, force an initial migration statement to fail and separately set `PRAGMA user_version` above the version supported by the binary.

Expected: migration failure leaves the previous version unchanged; a newer schema is rejected without destructive changes.

## Platform Check

Build and run informational commands on macOS and Linux. On POSIX, verify newly created directory/file permissions are owner-only. Do not use a developer's real `~/.worklog` for destructive migration scenarios.
