# Feature Specification: Initialize CLI and Local Storage

**Feature**: `01-initiate-cli-and-local-storage`  
**Status**: Ready for implementation planning  
**Source**: `goal.md`, `../../BRD.md`, `../../PRD.md`, `../../ERD.md`

## Summary

Provide the first usable `wl` command-line application. A developer can run the application, read its help and version, and use a private local application directory containing a default configuration and a migrated SQLite database. This feature establishes storage and CLI foundations for later worklog features; it does not implement session, note, Git capture, or AI commands.

## User Scenarios and Acceptance

### P1 — Start the application for the first time

As a developer, I want the CLI to prepare its local files automatically so I can start using later worklog commands without manually creating a database or configuration file.

**Acceptance scenarios**

1. Given a writable home directory with no `~/.worklog`, when I run `wl` without a subcommand, then the application creates `~/.worklog/`, `config.yaml`, and `worklog.db` and displays a useful initial status or readiness message.
2. Given the files already exist, when I run `wl` again, then existing configuration and worklog data are preserved and startup completes without duplicating schema objects.
3. Given the local directory or database cannot be created, when startup runs, then `wl` exits non-zero and reports the failing path and a corrective hint without printing secrets.

### P1 — Inspect help and version

As a developer, I want standard help and version output so I can understand the CLI and identify the installed build.

**Acceptance scenarios**

1. When I run `wl --help` or `wl -h`, then the output identifies `wl`, describes its purpose, and shows available commands and global flags.
2. When I run `wl --version` or `wl version`, then the output includes a stable version string; source builds without release metadata report `dev`.
3. Help and version remain available when the database is missing or the home directory is read-only; these informational invocations do not require opening SQLite.

### P1 — Initialize and migrate local storage

As a developer, I want the initial schema to be created safely so future worklog features can store data locally.

**Acceptance scenarios**

1. Given a fresh database, when application startup completes, then the schema contains the MVP entities and indexes defined by `data-model.md` and the ERD.
2. Given an already initialized database at the current schema version, when startup runs, then no migration is re-applied and existing rows remain unchanged.
3. Given a database at an older supported schema version, when startup runs, then pending migrations are applied in order and the schema version advances only after each migration succeeds.
4. Given a migration fails, when startup runs, then the migration is rolled back, the failure is reported, and the prior schema version remains recorded.
5. Given a database created by a newer unsupported application version, when an older `wl` starts, then it fails clearly without attempting a downgrade or destructive change.

### P2 — Keep local worklog data private

As a developer, I want local files to be private to my user account so work context is not exposed to other local users by default.

**Acceptance scenarios**

1. When the application creates the data directory and files on a platform supporting POSIX permissions, then it requests owner-only access for the directory and files.
2. When an existing configuration is loaded, then defaults are added only for missing values and user-selected values are retained.
3. When defaults are first written, then they match the documented configuration contract and do not contain credentials or secrets.

## Functional Requirements

- **FR-001**: The executable name is `wl`.
- **FR-002**: The CLI provides a root command that succeeds with no arguments and initializes local configuration and storage before reporting readiness.
- **FR-003**: The CLI provides standard help output through `--help`, `-h`, and the built-in help command.
- **FR-004**: The CLI provides `--version` and a `version` command. Release metadata may override the version at build time; its fallback value is `dev`.
- **FR-005**: The application resolves its home directory through the operating system and creates `<home>/.worklog/` without hard-coded usernames or shell expansion.
- **FR-006**: First-run initialization creates `config.yaml` and `worklog.db` in the data directory. Repeated initialization preserves existing user data.
- **FR-007**: The default database setting resolves to `<home>/.worklog/worklog.db`; path resolution supports the documented `~` shorthand without writing an unresolved tilde path.
- **FR-008**: The default configuration includes the database path, ticket-key pattern, Git metadata capture options, and AI privacy defaults described in `contracts/cli.md`.
- **FR-009**: Full Git diff capture and transmission are disabled by default. No API key or credential is stored in the config file or SQLite database by this feature.
- **FR-010**: Database access uses SQLite and enables foreign-key enforcement for application connections.
- **FR-011**: The initial schema is derived from `docs/ERD.md`: `tickets`, `work_sessions`, `work_activities`, and `ai_generations`, including stated checks, foreign keys, uniqueness rules, and indexes.
- **FR-012**: Schema migrations are versioned, embedded with the application, and safely repeatable at startup. A failed migration must not be recorded as applied.
- **FR-013**: The storage boundary follows the repository's top-level packages: `domain/`, `application/`, `infrastructure/`, `delivery/`, and `cmd/wlog/`; this feature does not add an `internal/` directory.
- **FR-014**: Interfaces are small and defined where consumed. This feature does not add a generic CRUD base repository; it establishes the SQLite connection and migration boundary for feature-specific repositories.
- **FR-015**: Startup failures return a non-zero process exit code and identify the failed operation without exposing credentials.
- **FR-016**: Help and version output do not depend on successful configuration loading or database initialization.
- **FR-017**: Local startup and informational commands do not require network access.

## Key Entities

- **Local configuration**: User-editable settings stored at `~/.worklog/config.yaml`.
- **SQLite database**: Local persistent store at the configured database path.
- **Schema version**: Monotonic migration version associated with the database; the application may use SQLite's application-owned `user_version` field.
- **Ticket**: Locally known Jira key, optionally with a title.
- **Work session**: A ticket-associated time interval with title, optional repository, timestamps, duration, and status.
- **Work activity**: A NOTE or GIT_COMMIT event; `session_id` is nullable for unsessioned commits, and `ticket_id` is nullable for unassigned activity.
- **AI generation**: A stored generated output associated with an optional ticket and a constrained generation type.

Field-level constraints and relationships are specified in `data-model.md`.

## Success Criteria

- **SC-001**: A first run with an empty home directory creates the data directory, config, and migrated database without requiring manual setup.
- **SC-002**: `wl --help` and `wl --version` each return successfully without needing a valid or writable database.
- **SC-003**: Running initialization repeatedly preserves config values and stored rows while leaving the schema at the same version.
- **SC-004**: A migration failure leaves the database at its prior schema version and reports an actionable error.
- **SC-005**: Fresh local startup completes within the PRD's 200 ms target on a typical developer machine, excluding initial filesystem or operating-system cold-start variance.
- **SC-006**: No network request is made during local initialization, help, or version handling.
- **SC-007**: The resulting CLI can be built and launched on macOS and Linux, the initial supported platforms in the BRD.

## Assumptions

- Running `wl` without a subcommand is the first-run initialization path; no separate `wl init` command is introduced by this feature.
- Informational commands (`--help`, `--version`) have no filesystem side effects. The generated files are guaranteed after ordinary root startup.
- The current ERD supersedes older schema snippets in the BRD/PRD where their entity fields differ.
- The existing Go module path in `go.mod` remains `github.com/taupikpirdian/wlog`.
- The PRD's proposed SQLite, Cobra, and Viper dependencies are the starting choices; exact dependency versions are resolved when implementation begins.
- On non-POSIX platforms, permission behavior is best-effort and documented; macOS and Linux are the initial supported targets.

## Out of Scope

- `start`, `stop`, `note`, `today`, `git`, `summary`, `description`, and hook-installation behavior.
- Git repository scanning, Jira API access, AI providers, credential management, export, and backup.
- A generic repository framework or repository methods for features that do not yet exist.
- Automatic updates, cloud synchronization, and Windows support.
