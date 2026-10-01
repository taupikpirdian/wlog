# Feature Specification: Ticket Management

**Feature**: `02-ticket-management`  
**Status**: Ready for implementation  
**Source**: `goal.md`, `../../BRD.md`, `../../PRD.md`, `../../ERD.md`

## Summary

Treat a Jira ticket key as the stable grouping identifier for worklog data. The application can parse a key from a commit message, create or find the corresponding local ticket, and upsert it without creating duplicate ticket records. This feature prepares later session, activity, and AI-generation flows to attach their data to the same ticket.

## User Scenarios and Acceptance

### P1 — Identify a ticket in a commit message

As a developer, I want the worklog to recognize Jira ticket keys in commit messages so coding activity can be grouped under its ticket.

**Acceptance scenarios**

1. Given the default ticket pattern, when the message is `fix: [OOT-3751][taupik.pirdian@salt.co.id][SP29] fix tax calculation`, then the extracted key is `OOT-3751`.
2. Given the default ticket pattern, when the message is `fix: [OOT-3751][taupik.pirdian@salt.co.id] fix tax calculation`, then the extracted key is `OOT-3751`.
3. Given the default ticket pattern, when the message is `fix: [taupik.pirdian@salt.co.id][OOT-3751] fix tax calculation`, then the extracted key is `OOT-3751`.
4. Given the default ticket pattern, when the message is `fix: [taupik.pirdian@salt.co.id][ORB-3751] fix tax calculation`, then the extracted key is `ORB-3751`.
5. Given a message containing no matching key, when extraction runs, then it returns “not found” and does not create a ticket.
6. Given a message containing more than one matching key, when extraction runs, then the first match from left to right is returned.

### P1 — Find an existing ticket

As a worklog feature, I want to find a ticket by its exact key so sessions and activities can refer to one stable ticket record.

**Acceptance scenarios**

1. Given a stored ticket `OOT-3751`, when the application finds `OOT-3751`, then it returns that ticket with its local ID, key, optional title, and timestamps.
2. Given no ticket with the requested key, when find runs, then it returns a not-found result and leaves storage unchanged.
3. Given a key that differs in case or characters, when find runs, then it does not silently return a different ticket.

### P1 — Create and upsert tickets

As a worklog feature, I want ticket records created or refreshed by key so repeated work on the same Jira issue does not create duplicate records.

**Acceptance scenarios**

1. Given a valid key that is not stored, when create runs, then one ticket is stored with that key and an optional title.
2. Given an already stored key, when create runs, then it returns a duplicate-ticket error and does not create a second row.
3. Given a valid key that is not stored, when upsert runs, then it creates and returns one ticket.
4. Given a stored key and a non-blank incoming title, when upsert runs, then it trims surrounding whitespace, updates the title and `updated_at`, and preserves the ID and `created_at`.
5. Given a stored key and a missing or whitespace-only incoming title, when upsert runs, then it preserves the existing title and timestamps.
6. Given repeated upserts with the same key and title, when each upsert runs, then exactly one ticket exists for the key.
7. Given an invalid key, when create or upsert runs, then the operation returns a validation error and storage remains unchanged.

### P2 — Group later worklog data under a ticket

As a developer, I want ticket identity shared across sessions, activities, and AI outputs so those records can be associated consistently.

**Acceptance scenarios**

1. When a ticket is created or found, then its stable local ID can be used by later session, activity, and AI-generation features.
2. When Git capture later extracts a key, then it can upsert the ticket without needing a prior manual ticket-registration step.
3. An activity without a recognized ticket remains unassigned; extraction does not invent or guess a ticket key.

## Functional Requirements

- **FR-001**: Ticket keys follow the configured `ticket.pattern`; the default is `[A-Z][A-Z0-9]+-[0-9]+`.
- **FR-002**: Ticket extraction searches the full commit message and returns the first matching substring from left to right.
- **FR-003**: Extraction preserves the matched key exactly and returns a distinct not-found result when there is no match.
- **FR-004**: Extraction of a key does not itself create or update a ticket record.
- **FR-005**: A ticket has a stable local ID, unique key, optional title, creation timestamp, and update timestamp as specified by the ERD.
- **FR-006**: Create validates a ticket key and fails clearly if the key already exists.
- **FR-007**: Find uses an exact ticket-key match and does not create data when no match exists.
- **FR-008**: Upsert creates a ticket when the key is absent and updates an existing record when the key is present.
- **FR-009**: Upsert replaces an existing title only when a non-blank title is supplied; it trims surrounding whitespace, and a missing or whitespace-only title preserves the stored title.
- **FR-010**: A successful upsert for an unchanged key/title does not create another row or change timestamps.
- **FR-011**: Ticket key uniqueness is enforced by the existing database schema as well as application behavior.
- **FR-012**: This feature uses the existing `tickets` table and does not introduce a schema migration.
- **FR-013**: Ticket operations are exposed as application behavior for later features; this feature does not add a standalone `wl ticket` command.
- **FR-014**: The ticket persistence interface is defined at its point of consumption, and the SQLite implementation remains in infrastructure.
- **FR-015**: Ticket management does not call Jira or any network service.
- **FR-016**: Ticket errors identify invalid, duplicate, and missing keys without exposing unrelated local data.

## Key Entities

- **Ticket**: Local representation of a Jira issue, keyed by a unique Jira ticket key. A title is optional because Git capture may first observe a key without additional ticket metadata.
- **Ticket key**: A string matching the configured pattern; the default matches examples such as `OOT-3751`, `ABC-123`, and `PROJECT1-999`.
- **Commit message**: Input text from which the first configured ticket-key match may be extracted. The message itself is not persisted by this feature.

Ticket relationships with work sessions, activities, and AI generations are defined in `data-model.md` and `docs/ERD.md`.

## Success Criteria

- **SC-001**: All four commit-message examples in `goal.md` return their expected ticket key.
- **SC-002**: A message without a valid key yields no key and causes no ticket record to be created.
- **SC-003**: Creating, finding, and upserting a key produce the behaviors described in the acceptance scenarios.
- **SC-004**: Repeated upserts for one key never create duplicate ticket records.
- **SC-005**: Later features can reference a ticket by stable local ID and associate their worklog records with it.
- **SC-006**: Ticket lookup and upsert are local operations and require no network connection.

## Assumptions

- The configured ticket regex is compiled and validated by startup configuration handling; ticket services receive a valid pattern.
- Matching is case-sensitive because the default pattern permits uppercase Jira project keys only.
- If multiple keys occur, the first textual match is selected. This is a deterministic default for commit messages containing secondary references.
- `Create`, `Find`, `Upsert`, and `Extract` are application capabilities used by other worklog features; no standalone ticket command was requested in `goal.md`.
- Upsert only changes the title when a non-blank title is supplied; surrounding title whitespace is trimmed. Ticket keys are not normalized or rewritten.
- Ticket history/search and fetching ticket metadata from Jira are outside this feature.

## Out of Scope

- Standalone CLI commands to list, show, edit, or delete tickets.
- Jira API integration or retrieval of remote ticket summaries/statuses.
- Searching ticket history or aggregating and rendering a full activity timeline.
- Session, note, Git-commit persistence, or AI-generation behavior beyond establishing ticket identity for those later features.
- Database schema changes; the existing ERD already defines the `tickets` table and unique key.
