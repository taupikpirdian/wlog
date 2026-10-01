# Research: Ticket Management

## Decision 1 — Use the existing ticket key rule

**Decision**: Use the configured `ticket.pattern`, whose default is `[A-Z][A-Z0-9]+-[0-9]+`.

**Rationale**: The exact pattern appears in the goal, BRD, PRD, and default config from feature 01. The goal's four examples match it, including keys in different positions among bracketed commit metadata.

**Alternatives considered**: Hard-code the regex in the parser or add ticket-project-specific rules. Hard-coding would ignore the existing configuration contract; additional rules are unsupported by current requirements.

## Decision 2 — Extract the first occurrence in message order

**Decision**: Search the entire commit message and return the first matching substring from left to right.

**Rationale**: All supplied examples contain one ticket key but place it in different positions. A first-match rule is deterministic and allows future commit flows to distinguish the primary ticket from secondary references.

**Alternatives considered**: Return all keys, choose the last key, or reject messages with multiple matches. Those behaviors are not requested and would complicate later automatic capture. If multi-ticket commits become a real workflow, the contract can evolve explicitly.

## Decision 3 — Keep ticket persistence on the existing schema

**Decision**: Implement create/find/upsert against the existing `tickets` table without a new migration.

**Rationale**: The ERD already defines `id`, unique `ticket_key`, nullable `title`, `created_at`, and `updated_at`. Foreign keys from sessions, activities, and AI generations already point to this table.

**Alternatives considered**: Add ticket metadata, status, or Jira fields. These are not present in the ERD or feature goal and Jira API integration is explicitly outside the MVP.

## Decision 4 — Expose use cases to later features, not a new CLI command

**Decision**: Provide ticket capabilities at the application layer for session and Git flows to consume; do not introduce `wl ticket` commands in this feature.

**Rationale**: The goal enumerates operations (`Create`, `Find`, `Upsert`, `Extract`) but names no end-user command. The BRD/PRD use ticket detection as part of work sessions and Git capture. A standalone command would invent a workflow and acceptance criteria.

**Alternatives considered**: Add `wl ticket add/show/list`. These commands have no defined user behavior in the source requirements and ticket-history search is listed as future roadmap.

## Decision 5 — Preserve known title data during automatic upsert

**Decision**: Upsert trims incoming title whitespace and updates an existing title only when the result is non-empty; identical values do not update timestamps.

**Rationale**: The ERD makes title nullable because a ticket can first appear in a commit message. Automatic capture must therefore be able to upsert a key without deleting a title entered by a later/manual flow. Trimming makes whitespace-only input behave as an absent title.

**Alternatives considered**: Always replace title, including with null/empty, or never update titles. The chosen rule preserves richer local metadata while allowing a supplied title to refresh it.
