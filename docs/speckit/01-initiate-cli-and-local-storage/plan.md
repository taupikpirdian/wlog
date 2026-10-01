# Implementation Plan: Initialize CLI and Local Storage

**Feature**: `01-initiate-cli-and-local-storage`  
**Status**: Implemented; compile and static analysis complete  
**Inputs**: `spec.md`, `goal.md`, `../../BRD.md`, `../../PRD.md`, `../../ERD.md`

## Technical Context

| Area | Decision |
|---|---|
| Runtime | Go; keep the existing module path in `go.mod` |
| CLI | Cobra root command; expose help and version without opening storage |
| Config | YAML at `~/.worklog/config.yaml`; use PRD defaults and preserve existing values |
| Database | SQLite through `database/sql`; use the PRD's pure-Go `modernc.org/sqlite` driver to avoid a CGO compiler requirement for macOS/Linux builds |
| Migrations | Embedded ordered SQL files; use SQLite `PRAGMA user_version` as the schema version and apply each migration transactionally |
| Packages | `cmd/wlog`, `delivery/cli`, `application`, `domain`, and `infrastructure/storage`; no `internal/` directory |
| Platforms | macOS and Linux initially |
| Network | Not required for initialization, help, or version |
| Testing | Acceptance scenarios are listed in `quickstart.md`; test execution belongs to implementation validation |

## Architecture

```text
cmd/wlog
  └── delivery/cli
       └── application/bootstrap
            ├── domain (interfaces/value types where needed)
            └── infrastructure/storage
                 ├── config
                 ├── sqlite connection
                 └── embedded migrations
```

The existing Clean Architecture direction remains `delivery → application → domain ← infrastructure`. `cmd/wlog` is the composition root. It constructs the bootstrap dependencies and passes them into the CLI. Cobra command declarations live under `delivery/cli`; storage details remain in `infrastructure/storage`.

Initialization should be idempotent and ordered:

1. Resolve the user's home directory and derive the default data/config/database paths.
2. Create the data directory with owner-only permissions where supported.
3. Create a default config only if it does not exist; otherwise load and validate it while preserving user values.
4. Open or create SQLite, enable foreign keys on every connection, and verify connectivity.
5. Read the schema version, reject a database from a newer unsupported version, then apply each pending embedded migration in its own transaction.
6. Close storage cleanly and print a concise readiness result.

The root command's startup hook runs steps 1–6 for ordinary no-argument invocation. Help and version use command metadata only and must bypass the bootstrap hook.

## Dependency Decisions

- **Cobra** is explicitly recommended by the PRD and provides the root command, built-in help, command tree, and flags.
- **modernc.org/sqlite** is selected over `mattn/go-sqlite3` to avoid requiring CGO and a C compiler on supported developer machines.
- **Viper** is the configuration library recommended by the PRD. Keep it behind the config adapter so the application layer does not depend on Viper types.
- **No separate migration dependency** is needed for the first schema. Embedded SQL plus a version marker keeps the bootstrap path small; migration atomicity is provided by SQLite transactions.
- **No generic base repository** is introduced. Add feature-level interfaces only when a use case consumes them, following the Go Clean Architecture skill.

## Constitution Check

No `.specify/memory/constitution.md` is present in this repository. Apply the available Clean Architecture skill and user preference:

- [x] Business rules remain outside CLI and SQLite code.
- [x] SQLite details remain in infrastructure.
- [x] Dependencies point inward; composition occurs at `cmd/wlog`.
- [x] Packages live at the repository root as requested; no `internal/` directory.
- [x] No generic `utils`, `common`, or generic CRUD repository package is added.
- [x] Local worklog data remains local; full diff and credentials are not included by default.

## Gate Evaluation

- **Scope gate**: Pass. This feature is limited to executable bootstrap, local config, initial schema, and informational CLI output.
- **Data gate**: Pass. Initial tables and constraints derive from `docs/ERD.md`.
- **Privacy gate**: Pass. Local data only; no credentials; full diff off by default.
- **Architecture gate**: Pass with top-level packages, respecting the user's explicit instruction not to use `internal/`.
- **Migration gate**: Pass. Migrations are versioned and transactionally applied; downgrades are not automatic.

## Phase 0 — Research Summary

See `research.md` for choices and alternatives. Key design outcome: use the dependency set already proposed by the PRD, with `modernc.org/sqlite` selected for a CGO-free local build and `PRAGMA user_version` to track schema upgrades.

## Phase 1 — Design Artifacts

- `data-model.md`: configuration, schema entities, constraints, and migration behavior.
- `contracts/cli.md`: command-line behavior, output, exit statuses, and configuration contract.
- `quickstart.md`: manual scenarios for first run, repeat run, help/version, and migration failure.

## Implementation Sequence

1. [x] Confirm module path and add Cobra, Viper, and SQLite driver dependencies.
2. [x] Create the CLI composition root and Cobra root with help/version metadata.
3. [x] Add config path resolution, defaults, first-run creation, validation, and safe file permissions.
4. [x] Add SQLite connection setup and foreign-key enforcement.
5. [x] Add embedded initial migration based on the ERD, schema-version checks, and transactional migration runner.
6. [x] Wire ordinary root startup to initialize config/database and show readiness.
7. [x] Build for macOS and Linux and run static analysis. Interactive quickstart scenarios were not run.

## Risks and Mitigations

- **Directory permissions differ by platform**: request owner-only modes and verify actual permissions on supported POSIX platforms.
- **SQLite pragmas can be connection-scoped**: configure foreign-key enforcement through the driver DSN/connection setup so pooled connections receive it consistently.
- **Home directory may be unavailable or read-only**: return a path-specific non-zero error; keep help/version usable.
- **Existing config may be malformed**: report the parse error and path; do not silently overwrite the user's file.
- **Schema may be newer than the CLI**: refuse startup without modifying the database.
- **No `.specify/` project scaffolding exists**: this plan and artifacts are placed in the user-provided `docs/speckit/` feature directory; the Spec Kit setup script workflow could not be run.
