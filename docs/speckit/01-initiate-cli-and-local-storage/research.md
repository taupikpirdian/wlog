# Research: Initialize CLI and Local Storage

## Decision 1 — Cobra for the CLI

**Decision**: Use Cobra for the root command and subsequent subcommands.

**Rationale**: The PRD names Cobra, and its command model includes standard help, flags, and version metadata. Keep command definitions in the delivery layer and leave `main` as the composition entry point.

**Alternatives considered**: Go's standard `flag` package; hand-written argument parsing. These reduce dependencies but would require maintaining help, subcommand dispatch, and future command behavior that the PRD already anticipates.

**Source**: [Cobra user guide](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md).

## Decision 2 — Pure-Go SQLite driver

**Decision**: Use `modernc.org/sqlite` with `database/sql`.

**Rationale**: The PRD already recommends this driver. Its documentation describes a CGo-free SQLite implementation and integration through `database/sql`, avoiding an external C compiler requirement for local developer builds.

**Alternatives considered**: `github.com/mattn/go-sqlite3`, which is a `database/sql` driver but requires CGO and a C compiler according to its project documentation. The added native build prerequisite is unnecessary for this MVP.

**Sources**: [modernc.org/sqlite documentation](https://modernc.org/sqlite/), [go-sqlite3 build requirements](https://github.com/mattn/go-sqlite3).

## Decision 3 — Version migrations using SQLite `user_version`

**Decision**: Track the applied migration number with `PRAGMA user_version`; embed ordered SQL migrations and apply one version at a time in a transaction.

**Rationale**: SQLite documents `user_version` as an application-owned integer, which makes it suitable for schema version tracking without adding a bookkeeping table beyond the entities in the ERD. SQLite supports explicit transactions; a failed migration can be rolled back before its version is advanced.

**Alternatives considered**: Add a `schema_migrations` table or introduce a migration library. Either can be appropriate as migration needs grow, but both add machinery for the single initial migration. Revisit if concurrent processes, checksums, or rollback workflows become requirements.

**Sources**: [SQLite `user_version` pragma](https://www.sqlite.org/pragma.html#pragma_user_version), [SQLite transactions](https://www.sqlite.org/lang_transaction.html).

## Decision 4 — Preserve the existing package preference

**Decision**: Keep architecture packages at the module root and do not create `internal/`.

**Rationale**: The user explicitly requested that files not be put under `internal/`. The available Go Clean Architecture skill supports separating domain, application, infrastructure, and delivery responsibilities; the restriction is satisfied with root-level layer folders.

**Alternatives considered**: The PRD's sample layout uses `internal/`. That sample is superseded by the user's later explicit package-layout preference.

## Decision 5 — Use Viper behind a configuration adapter

**Decision**: Use the PRD-recommended Viper library only within infrastructure/configuration code; expose a typed application configuration value.

**Rationale**: It matches the existing PRD recommendation and YAML format. Keeping the dependency behind an adapter prevents it from spreading into domain and application packages.

**Alternatives considered**: Direct YAML marshaling or a handwritten parser. These may be simpler for defaults-only config, but the PRD anticipates configurable settings; reassess if the implementation adds avoidable complexity.

**Source**: [Cobra guide section on Viper configuration](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md).

## Open Implementation Checks

- Resolve current compatible module versions when dependencies are added; this plan intentionally does not pin moving latest versions.
- Verify SQLite foreign-key enforcement for each connection created by the chosen driver configuration.
- Verify restrictive file modes on macOS and Linux and document behavior on any additional platform.
- Keep help/version independent of config and database access.
