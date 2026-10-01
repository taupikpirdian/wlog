# Implementation Plan: Ticket Management

**Feature**: `02-ticket-management`  
**Status**: Implemented; build and static analysis complete  
**Inputs**: `spec.md`, `goal.md`, `../../BRD.md`, `../../PRD.md`, `../../ERD.md`

## Technical Context

| Area | Decision |
|---|---|
| Ticket key rules | Compile and use the `ticket.pattern` loaded by feature 01; default remains `[A-Z][A-Z0-9]+-[0-9]+` |
| Extraction | Search the full message and return the first match from left to right |
| Persistence | Use existing SQLite `tickets` table; no migration |
| Architecture | Domain ticket type, application service/port, infrastructure SQLite adapter; no `internal/` directory |
| CLI | No standalone ticket command; later session and Git commands consume the application behavior |
| Network | None; no Jira requests |
| Validation | Acceptance scenarios and repository behavior are documented in `quickstart.md` |

## Architecture

```text
delivery/cli (future start/git command)
       ↓
application/ticket (create, find, upsert, extract orchestration)
       ↓                         ↓
domain/ticket              infrastructure/storage
                                  └── SQLite tickets table
```

The ticket key parser belongs with domain-level ticket rules because it applies a business identifier format and has no storage dependency. The application service validates requests, coordinates parsing and persistence behavior, and defines the `TicketStore` port it consumes. The infrastructure adapter implements that port with parameterized SQLite statements. The composition root passes the adapter to the application service.

The application service offers four behaviors:

1. `ExtractKey(message)` returns the first matching key or an explicit not-found result.
2. `Create(key, title)` validates and inserts, returning a duplicate error when the key exists.
3. `FindByKey(key)` returns one exact match or a not-found error.
4. `Upsert(key, title)` inserts when missing and updates the trimmed title only when it is non-blank; unchanged input does not touch timestamps.

## Data and Persistence

- `tickets.id` is the stable foreign-key target used by subsequent worklog features.
- `ticket_key` remains unique as enforced by the current SQLite schema.
- `title` remains nullable.
- Use parameterized SQL; map SQLite unique-key violations to a domain/application duplicate error in `Create`.
- Upsert should preserve the existing row ID and `created_at`. Trim title whitespace and update `updated_at` only when the stored title changes.
- No schema migration is needed because feature 01 creates the entity and unique constraint from `docs/ERD.md`.

## Constitution Check

No `.specify/memory/constitution.md` is present. Apply Clean Architecture guidance and the user's established package preference:

- [x] Domain validation and extraction have no SQLite or CLI dependency.
- [x] Application use cases depend on the `TicketStore` abstraction.
- [x] SQLite statements and error translation stay in infrastructure.
- [x] Future delivery commands call application behavior rather than storage directly.
- [x] No `internal/`, generic CRUD base, or unrelated CLI commands are introduced.
- [x] Ticket parsing does not guess a key when there is no match.

## Gate Evaluation

- **Scope gate**: Pass. Only ticket identity, persistence operations, and commit-message extraction are included.
- **Schema gate**: Pass. Existing `tickets` table and key uniqueness satisfy the feature; no migration is required.
- **Behavior gate**: Pass with explicit assumptions for multiple matches and optional title updates.
- **Architecture gate**: Pass with root-level packages and a feature-specific storage port.
- **Integration gate**: Pass. The result is ready to be consumed by session and Git capture features without adding a public ticket command.

## Phase 0 — Research Summary

See `research.md`. The regex and ticket schema are already defined by BRD/PRD/ERD. The key implementation choices are deterministic first-match extraction and idempotent title-aware upsert.

## Phase 1 — Design Artifacts

- `data-model.md`: Ticket fields, uniqueness, and associations with future worklog records.
- `contracts/ticket-service.md`: Application operations and their outcomes/errors.
- `quickstart.md`: Scenarios covering examples, no-match, duplicate create, exact find, and upsert.

## Implementation Sequence

1. [x] Add a domain Ticket type and key validation/extraction behavior.
2. [x] Define small application-facing persistence behavior and ticket service methods.
3. [x] Implement SQLite create/find/upsert against the existing `tickets` table.
4. [x] Map unique constraint, missing row, and invalid key outcomes to stable errors.
5. [x] Expose constructors for composition by future session and Git CLI features; this feature adds no ticket command.
6. [ ] Run the acceptance scenarios in `quickstart.md` (not run in this implementation turn).

## Risks and Mitigations

- **Custom patterns may match surprising substrings**: use the configured regex exactly and document first-match behavior; retain the default rule from the PRD.
- **Multiple ticket IDs may occur in one commit**: consistently select the first left-to-right match.
- **Title may be absent during automatic capture**: keep title nullable and do not erase an existing title when upserting without one.
- **SQLite unique errors vary by driver**: isolate duplicate detection in the infrastructure adapter and expose a stable application error.
- **Feature 01 currently initializes and closes its database connection**: extend the storage composition so the ticket adapter opens the same configured database after initialization; do not duplicate schema setup.

## Repository Layout

```text
domain/ticket/                 Ticket entity, key validation, extraction
application/ticket/            Ticket service and consumed persistence port
infrastructure/storage/        SQLite ticket adapter
```

These packages are top-level as requested; no `internal/` directory is used.
