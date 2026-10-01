# Quickstart Validation: Ticket Management

## Prerequisites

- Go 1.22 or newer.
- Feature 01 local storage implementation and its initial SQLite migration.
- The ticket pattern configured as `[A-Z][A-Z0-9]+-[0-9]+` unless a scenario overrides it.

## Scenario 1 — Extract keys from the goal examples

Pass each message from `goal.md` to the ticket extractor:

```text
fix: [OOT-3751][taupik.pirdian@salt.co.id][SP29] fix tax calculation
fix: [OOT-3751][taupik.pirdian@salt.co.id] fix tax calculation
fix: [taupik.pirdian@salt.co.id][OOT-3751] fix tax calculation
fix: [taupik.pirdian@salt.co.id][ORB-3751] fix tax calculation
```

Expected results, in order: `OOT-3751`, `OOT-3751`, `OOT-3751`, `ORB-3751`.

## Scenario 2 — No match and multiple matches

Use one message without a key and one message with two valid keys.

Expected: no-match returns an empty/not-found result without an error or database write; multiple matches return the first key from left to right.

## Scenario 3 — Create and find

Create a new ticket, then find it by its exact key.

Expected: create returns one stable ID; find returns the same ID, key, optional title, and timestamps. Find with another key returns not-found without creating a row.

## Scenario 4 — Duplicate create and idempotent upsert

Create the same key twice, then upsert that key with no title and with the same title.

Expected: the second create returns duplicate; upserts return the original ID and do not create rows or alter timestamps when values are unchanged.

## Scenario 5 — Title update

Create a key without a title. Upsert with a title, then upsert again without one.

Expected: the title is added on the first upsert and preserved on the second. `created_at` and ID remain stable; `updated_at` changes only when the title changes.

## Scenario 6 — Invalid key

Attempt create/upsert using an empty key and a key that does not match the active pattern.

Expected: validation error and no database changes.

## Regression Check

Confirm the existing schema version is unchanged and the current `tickets` table unique constraint is sufficient. No migration should be generated for this feature.
